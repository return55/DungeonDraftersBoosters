package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	_ "image/png"

	"github.com/gocolly/colly"
	"github.com/xuri/excelize/v2"
)

const baseURL string = "https://dungeondrafters.wiki.gg"
const imagesDirecotry string = "Images"

type Card struct {
	Archetype     string
	ImageName     string
	Name          string
	Rarity, Level int
	Description   string
}

func getColumnNames() [6]string {
	return [6]string{"Archetype", "Image", "Name", "Rarity", "Level", "Description"}
}

func initializeCollectors() (*colly.Collector, *colly.Collector) {
	c := colly.NewCollector()
	/*c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})
	c.OnResponse(func(r *colly.Response) {
		fmt.Println(r.StatusCode)
	})*/
	//Used to recover info from the booster page
	c2 := c.Clone()
	/*c2.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting", r.URL)
	})
	c2.OnResponse(func(r *colly.Response) {
		fmt.Println(r.StatusCode)
	})*/
	return c, c2
}

func downloadImage(imagePartialURL string) string {
	tmp := strings.Split(imagePartialURL, "/")
	fileName := tmp[len(tmp)-1]

	response, e := http.Get(baseURL + imagePartialURL)
	if e != nil {
		log.Fatal(e)
	}
	defer response.Body.Close()

	//open a file for writing
	file, err := os.Create(imagesDirecotry + string(os.PathSeparator) + fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Use io.Copy to just dump the response body to the file. This supports huge files
	_, err = io.Copy(file, response.Body)
	if err != nil {
		log.Fatal(err)
	}

	return fileName
}

func collectCardsData(c, c2 *colly.Collector) map[string][]Card {
	var boosters = make(map[string][]Card)
	//Here will be stored all cards images
	if _, err := os.Stat(imagesDirecotry); os.IsNotExist(err) {
		os.Mkdir(imagesDirecotry, 0777)
	}

	c.OnHTML("table", func(e *colly.HTMLElement) {
		e.ForEach(`tbody > tr > td > a`, func(_ int, el *colly.HTMLElement) {
			link := el.Attr("href")
			if !strings.Contains(link, ".png") {
				err := c2.Visit(baseURL + link)
				if err != nil {
					panic("error: " + err.Error())
				}
			}
		})
	})
	c2.OnHTML("div[class=mw-parser-output]", func(el *colly.HTMLElement) {
		var boosterNames []string
		el.ForEach(`h1 > span`, func(_ int, ele *colly.HTMLElement) {
			boosterNames = append(boosterNames, ele.Text)
			//Must eliminate special characters for Excel
			for i := range boosterNames {
				boosterNames[i] = regexp.MustCompile(`[^a-zA-Z0-9 ]+`).
					ReplaceAllString(boosterNames[i], "_")
			}
		})
		el.ForEach(`table > tbody > tr`, func(_ int, ele *colly.HTMLElement) {
			//get rarity
			re := regexp.MustCompile("[0-9]+")
			rarityImagePath := ele.ChildAttr("td:nth-child(4) > a", "href")
			rarity := re.FindAllString(rarityImagePath, -1)
			if len(rarity) != 1 {
				//fmt.Println("The row does not contain the info about a card.")
			} else {
				//convert to int the rarity of the card
				rarityInt, err := strconv.Atoi(rarity[0])
				if err != nil {
					panic("Rarity conversion error:" + err.Error())
				}
				//get the level of the card
				levelImagePath := ele.ChildAttr("td:nth-child(5) > div > a ", "href")
				level := re.FindAllString(levelImagePath, -1)
				levelInt, err := strconv.Atoi(level[0])
				if err != nil {
					panic("Level conversion error:" + err.Error())
				}
				//get the image of the card
				imagePartialURL := ele.ChildAttr("td:nth-child(2) > div > a > img", "src")
				imageFileName := downloadImage(imagePartialURL)
				//tmp := strings.Split(imagePartialURL, "/")
				//imageFileName := tmp[len(tmp)-1]
				//get all the other info
				c := Card{
					Archetype:   ele.ChildAttr("td > a", "title"),
					ImageName:   imageFileName,
					Name:        ele.ChildText("td:nth-child(3)"),
					Rarity:      rarityInt,
					Level:       levelInt,
					Description: ele.ChildText("td:nth-child(6)"),
				}
				//in order to add the card to the booster or its expansion, chech
				//the Archetype.
				if c.Archetype == "Stranger" && len(boosterNames) > 1 {
					boosters[boosterNames[1]] = append(boosters[boosterNames[1]], c)
				} else {
					boosters[boosterNames[0]] = append(boosters[boosterNames[0]], c)
				}
			}
		})
	})

	c.Visit("https://dungeondrafters.wiki.gg/wiki/Boosters")
	return boosters
}

func printBoosters(boosters map[string][]Card) {
	count := 0
	for booster, cards := range boosters {
		fmt.Println(booster)
		for _, card := range cards {
			fmt.Println("    " + card.Name)
			count++
		}
	}
	fmt.Println("Cards number = ", count)
	fmt.Println(boosters["Ruins of Garada"][7])
}

func createSheet(f *excelize.File, style int, boosterName string) {
	f.NewSheet(boosterName)
	if err := f.SetColWidth(boosterName, "F", "F", 59); err != nil {
		panic(err)
	}
	if err := f.SetColWidth(boosterName, "A", "C", 15); err != nil {
		panic(err)
	}
	if err := f.SetRowStyle(boosterName, 1, 200, style); err != nil {
		panic(err)
	}
}

func insertColumnNames(f *excelize.File, boosterName string) {
	for col, colName := range getColumnNames() {
		if err := f.SetCellValue(boosterName, fmt.Sprintf("%c", col+65)+strconv.Itoa(1),
			colName); err != nil {
			panic(err)
		}
	}
}

func main() {
	//Used to recover the URL of single boosters
	c, c2 := initializeCollectors()

	//Collect cards data
	boosters := collectCardsData(c, c2)

	//Print boosters info
	//printBoosters(boosters)
	f := excelize.NewFile()

	style, err := f.NewStyle(
		&excelize.Style{
			Alignment: &excelize.Alignment{
				Vertical:   "center",
				Horizontal: "center",
				WrapText:   true,
			},
		},
	)
	if err != nil {
		panic(err)
	}

	for boosterName, cards := range boosters {
		createSheet(f, style, boosterName)
		insertColumnNames(f, boosterName)
		for row, card := range cards {
			if err := f.SetRowHeight(boosterName, row+2, 59); err != nil {
				panic(err)
			}
			col := 0
			f.SetCellValue(boosterName, fmt.Sprintf("%c", col+65)+strconv.Itoa(row+2),
				card.Archetype)
			col++
			if err := f.AddPicture(boosterName, fmt.Sprintf("%c", col+65)+strconv.Itoa(row+2),
				imagesDirecotry+string(os.PathSeparator)+card.ImageName,
				&excelize.GraphicOptions{OffsetX: 32, OffsetY: 13}); err != nil {
				if err := f.SaveAs("Book1.xlsx"); err != nil {
					fmt.Println(err)
				}
				panic(err)
			}
			col++
			f.SetCellValue(boosterName, fmt.Sprintf("%c", col+65)+strconv.Itoa(row+2),
				card.Name)
			col++
			f.SetCellValue(boosterName, fmt.Sprintf("%c", col+65)+strconv.Itoa(row+2),
				card.Rarity)
			col++
			f.SetCellValue(boosterName, fmt.Sprintf("%c", col+65)+strconv.Itoa(row+2),
				card.Level)
			col++
			f.SetCellValue(boosterName, fmt.Sprintf("%c", col+65)+strconv.Itoa(row+2),
				card.Description)
		}
	}

	if err := f.SaveAs("DungeonDrafters_CardsChecklist.xlsx"); err != nil {
		fmt.Println(err)
	}

}
