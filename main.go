package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/machinebox/graphql"
)

func main() {
	dump("players", getNMostSubscribedPlayers(1500))
	p, _ := pick[[]playersub]("players")
	randomDate := time.Date(2023, time.May, 0, 0, 0, 0, 0, time.Local)
	index := (int(time.Now().Sub(randomDate).Hours()) / 24) % len(p)
	fmt.Println(p[index])

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.Default())

	r.LoadHTMLGlob("./*.html")
	r.GET("/", func(c *gin.Context) {
		newDate := (int(time.Now().Sub(randomDate).Hours()) / 24) % len(p)
		if newDate != index {
			index = newDate
		}
		c.HTML(http.StatusOK, "index.html", nil)
	})
	r.GET("/player", func(c *gin.Context) {
		player := c.DefaultQuery("player", "")
		trys, _ := strconv.Atoi(c.DefaultQuery("trys", "0"))
		res, _ := comparePlayerInformations(p[index].Slug, player, trys)
		c.Data(http.StatusOK, "text/html; charset=utf-8", res.Bytes())
	})
	r.GET("/all-players", func(c *gin.Context) {
		var res bytes.Buffer
		for _, player := range p {
			res.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, player.Slug, player.DisplayName))
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", res.Bytes())
	})
	r.Run()
}

type league struct {
	Football struct {
		Leagues []struct {
			Slug   string `json:"slug"`
			Format string `json:"format"`
		} `json:"leaguesOpenForGameStats"`
	} `json:"football"`
}

type club struct {
	Football struct {
		Club struct {
			ActivePlayers struct {
				Nodes []struct {
					Slug          string `json:"slug"`
					Subscriptions int    `json:"subscriptionsCount"`
					DisplayName   string `json:"displayName"`
				} `json:"nodes"`
			} `json:"activePlayers"`
		} `json:"club"`
	} `json:"football"`
}

type competition struct {
	Football struct {
		Competition struct {
			Clubs struct {
				Nodes []struct {
					Slug string `json:"slug"`
				} `json:"nodes"`
			} `json:"clubs"`
		} `json:"competition"`
	} `json:"football"`
}

type playersub struct {
	Slug          string
	Subscriptions int
	DisplayName   string
}

type playerInfos struct {
	Football struct {
		Player struct {
			Age         int     `json:"age"`
			Position    string  `json:"position"`
			ShirtNumber int     `json:"shirtNumber"`
			PictureUrl  string  `json:"pictureUrl"`
			L5          float32 `json:"l5"`
			L15         float32 `json:"l15"`
			DisplayName string  `json:"displayName"`
			ActiveClub  struct {
				PictureUrl     string `json:"pictureUrl"`
				DomesticLeague struct {
					Slug string `json:"slug"`
				} `json:"domesticLeague"`
			} `json:"activeClub"`
			Country struct {
				FlagUrl string `json:"flagUrl"`
			} `json:"country"`
		} `json:"player"`
	} `json:"football"`
}

type playerinf struct {
	Age          int
	Position     string
	ShirtNumber  int
	Club         string
	ClubLeague   string
	NationalTeam string
	Slug         string
	L5           int
	L15          int
	PicUrl       string
	Name         string
}

type color string

const (
	RED    color = "red"
	YELLOW color = "yellow"
	GREEN  color = "green"
)

type arrow int

const (
	UNDER arrow = iota
	NONE
	OVER
)

func getShortPosition(pos string) string {
	switch pos {
	case "Goalkeeper":
		return "GK"
	case "Defender":
		return "DEF"
	case "Midfielder":
		return "MID"
	case "Forward":
		return "FWD"
	}
	return "ERR"
}

func dump(filename string, data any) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Fatal("Error encoding cache" + err.Error())
	}
	f, err := os.Create(filename + ".bin")
	if err != nil {
		log.Fatal("Couldn't open cache file " + err.Error())
	}
	defer f.Close()
	f.Write(buf.Bytes())
}

func pick[K interface{}](filename string) (K, error) {
	var ret K
	f, err := os.ReadFile(filename + ".bin")
	if err != nil {
		return ret, err
	}
	dec := gob.NewDecoder(bytes.NewReader(f))
	err = dec.Decode(&ret)
	if err != nil {
		return ret, err
	}
	return ret, nil
}

func comparePlayerInformations(slug1, slug2 string, trys int) (bytes.Buffer, bool) {
	var ret bytes.Buffer
	ch := make(chan playerinf, 2)
	go getPlayerInformations(slug1, ch)
	go getPlayerInformations(slug2, ch)
	p1 := <-ch
	p2 := <-ch
	if p1.Slug != slug1 {
		p1, p2 = p2, p1
	}
	if p2.Age == 0 {
		ret.WriteString(`<div class="error" id="error">Error : Please pick a player in the list</div>`)
		return ret, false
	}
	ret.WriteString(`<div class="row">`)
	winner := true
	ret.WriteString(`<div><img src="` + p2.PicUrl + `" title="` + p2.Name + `"/></div>`)
	if p1.Age == p2.Age {
		ret.Write(buildTextDiv(GREEN, strconv.Itoa(p2.Age), NONE))
	} else if p1.Age > p2.Age {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.Age), OVER))
		winner = false
	} else if p1.Age < p2.Age {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.Age), UNDER))
		winner = false
	}
	if p1.Club == p2.Club {
		ret.Write(buildTextDiv(GREEN, `<img src="`+p2.Club+`"/>`, NONE))
	} else if p1.ClubLeague == p2.ClubLeague {
		ret.Write(buildTextDiv(YELLOW, `<img src="`+p2.Club+`"/>`, NONE))
		winner = false
	} else {
		ret.Write(buildTextDiv(RED, `<img src="`+p2.Club+`"/>`, NONE))
		winner = false
	}
	if p1.NationalTeam == p2.NationalTeam {
		ret.Write(buildTextDiv(GREEN, `<img src="`+p2.NationalTeam+`"/>`, NONE))
	} else {
		ret.Write(buildTextDiv(RED, `<img src="`+p2.NationalTeam+`"/>`, NONE))
		winner = false
	}
	if p1.ShirtNumber == p2.ShirtNumber {
		ret.Write(buildTextDiv(GREEN, strconv.Itoa(p2.ShirtNumber), NONE))
	} else if p1.ShirtNumber > p2.ShirtNumber {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.ShirtNumber), OVER))
		winner = false
	} else if p1.ShirtNumber < p2.ShirtNumber {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.ShirtNumber), UNDER))
		winner = false
	}
	if p1.Position == p2.Position {
		ret.Write(buildTextDiv(GREEN, p2.Position, NONE))
	} else {
		ret.Write(buildTextDiv(RED, p2.Position, NONE))
		winner = false
	}
	if p1.L5 == p2.L5 {
		ret.Write(buildTextDiv(GREEN, strconv.Itoa(p2.L5), NONE))
	} else if p1.L5 > p2.L5 {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.L5), OVER))
		winner = false
	} else if p1.L5 < p2.L5 {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.L5), UNDER))
		winner = false
	}
	if p1.L15 == p2.L15 {
		ret.Write(buildTextDiv(GREEN, strconv.Itoa(p2.L15), NONE))
	} else if p1.L15 > p2.L15 {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.L15), OVER))
		winner = false
	} else if p1.L15 < p2.L15 {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.L15), UNDER))
		winner = false
	}
	ret.WriteString("</div>")
	if winner {
		ret.WriteString(fmt.Sprintf(`
			<div class="winner" id="winner">
				<h2>Good Job ! You found <span>%s</span> in %d trys !
			</div>
		`, p2.Name, trys))
	}
	return ret, winner
}

func buildTextDiv(c color, content string, a arrow) []byte {
	var ret bytes.Buffer
	ret.WriteString(fmt.Sprintf(`<div class="%s">%s`, c, content))
	if a == OVER {
		ret.WriteString(`
		<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" fill="#DCDCDC" class="bi bi-caret-up-fill" viewBox="0 0 16 16">
			<path d="m7.247 4.86-4.796 5.481c-.566.647-.106 1.659.753 1.659h9.592a1 1 0 0 0 .753-1.659l-4.796-5.48a1 1 0 0 0-1.506 0z"/>
		</svg>`)
	} else if a == UNDER {
		ret.WriteString(`
		<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" fill="#DCDCDC" class="bi bi-caret-down-fill" viewBox="0 0 16 16">
			<path d="M7.247 11.14 2.451 5.658C1.885 5.013 2.345 4 3.204 4h9.592a1 1 0 0 1 .753 1.659l-4.796 5.48a1 1 0 0 1-1.506 0z"/>
		</svg>`)
	}
	ret.WriteString("</div>")
	return ret.Bytes()
}

func getPlayerInformations(slug string, res chan playerinf) {
	q := graphql.NewRequest(`
	query($slug: String!) {
		football {
			player(slug:$slug) {
				age
				position
				shirtNumber
				pictureUrl
				displayName
				l5: averageScore(type:LAST_FIVE_SO5_AVERAGE_SCORE)
        		l15: averageScore(type:LAST_FIFTEEN_SO5_AVERAGE_SCORE)
				activeClub {
					pictureUrl
					domesticLeague {
						slug
					}
				}
				country {
					flagUrl
				}
			}
		}
	}
	`)
	q.Var("slug", slug)
	player, err := callSorareApi[playerInfos](q)
	var ret playerinf
	if err == nil {
		ret.Age = player.Football.Player.Age
		ret.Club = player.Football.Player.ActiveClub.PictureUrl
		ret.ClubLeague = player.Football.Player.ActiveClub.DomesticLeague.Slug
		ret.NationalTeam = player.Football.Player.Country.FlagUrl
		ret.Position = getShortPosition(player.Football.Player.Position)
		ret.ShirtNumber = player.Football.Player.ShirtNumber
		ret.Slug = slug
		ret.L5 = int(player.Football.Player.L5)
		ret.L15 = int(player.Football.Player.L15)
		ret.Name = player.Football.Player.DisplayName
		ret.PicUrl = player.Football.Player.PictureUrl
	}
	res <- ret
}

func getNMostSubscribedPlayers(n int) []playersub {
	leagues := getAllLeagues()
	var clubs []string
	wg := sync.WaitGroup{}
	mu := &sync.Mutex{}
	for _, l := range leagues {
		wg.Add(1)
		go func(slug string) {
			defer wg.Done()
			c := getAllClubsFromCompetition(slug)
			mu.Lock()
			clubs = append(clubs, c...)
			mu.Unlock()
		}(l)
	}
	wg.Wait()
	var players []playersub
	for _, c := range clubs {
		wg.Add(1)
		go func(slug string) {
			defer wg.Done()
			p := getPlayersFromClub(slug)
			mu.Lock()
			players = append(players, p...)
			mu.Unlock()
		}(c)
	}
	wg.Wait()
	sort.Slice(players, func(i, j int) bool {
		return players[i].Subscriptions > players[j].Subscriptions
	})
	return players[:n]
}

func callSorareApi[K interface{}](req *graphql.Request) (K, error) {
	api_key := os.Getenv("SORARE_API_KEY")
	client := graphql.NewClient("https://api.sorare.com/graphql")
	req.Header.Set("APIKEY", api_key)
	var ret K
	if err := client.Run(context.Background(), req, &ret); err != nil {
		return ret, err
	}
	return ret, nil
}

func getAllLeagues() []string {
	q := `
	{
		football {
			leaguesOpenForGameStats
			{
			  slug
			  format
			}
		}
	  }
	`
	leagues, _ := callSorareApi[league](graphql.NewRequest(q))
	var ret []string
	for _, l := range leagues.Football.Leagues {
		if l.Format == "DOMESTIC_LEAGUE" {
			ret = append(ret, l.Slug)
		}
	}
	return ret
}

func getAllClubsFromCompetition(slug string) []string {
	q := graphql.NewRequest(`
	query($slug: String!) {
		football {
			competition(slug:$slug) {
				clubs {
					nodes {
						slug
					}
				}
			}
		}
	}
	`)
	q.Var("slug", slug)
	res, _ := callSorareApi[competition](q)
	var ret []string
	for _, c := range res.Football.Competition.Clubs.Nodes {
		ret = append(ret, c.Slug)
	}
	return ret
}

func getPlayersFromClub(slug string) []playersub {
	q := graphql.NewRequest(`
	query($slug: String!) {
		football {
			club(slug:$slug) {
				activePlayers {
					nodes {
						slug
						subscriptionsCount
						displayName
					}
				}
			}
		}
	}
	`)
	q.Var("slug", slug)
	res, _ := callSorareApi[club](q)
	var ret []playersub
	for _, p := range res.Football.Club.ActivePlayers.Nodes {
		ret = append(ret, playersub{Slug: p.Slug, Subscriptions: p.Subscriptions, DisplayName: p.DisplayName})
	}
	return ret
}
