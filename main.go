package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/machinebox/graphql"
)

var numberOfFound = 0

func main() {
	p, _ := pick[[]playersub]("players")
	loc, _ := time.LoadLocation("Europe/Paris")
	randomDate := time.Date(2023, time.May, 0, 0, 0, 0, 0, loc)
	index := (int(time.Now().In(loc).Sub(randomDate).Hours()) / 24) % len(p)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.Default())

	r.LoadHTMLGlob("./*.html")
	r.GET("/", func(c *gin.Context) {
		newIndex := (int(time.Now().In(loc).Sub(randomDate).Hours()) / 24) % len(p)
		if newIndex != index {
			index = newIndex
			numberOfFound = 0
			fmt.Println(p[index])
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
	r.GET("/nb-players", func(c *gin.Context) {
		var res bytes.Buffer
		res.WriteString(fmt.Sprintf("<h2>Today %d people found !</h2>", numberOfFound))
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
					CardSupply    []struct {
						Limited int `json:"limited"`
					} `json:"cardSupply"`
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
				Code    string `json:"code"`
			} `json:"country"`
		} `json:"player"`
	} `json:"football"`
}

type playerinf struct {
	Age              int
	Position         string
	ShirtNumber      int
	Club             string
	ClubLeague       string
	NationalTeam     string
	Slug             string
	L5               int
	L15              int
	PicUrl           string
	Name             string
	NationalTeamCode string
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

type Continent int

const (
	Asia Continent = iota
	Europe
	Africa
	Oceania
	Americas
	Other
)

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
	winner := slug1 == slug2
	ret.WriteString(`<div><img src="` + p2.PicUrl + `" title="` + p2.Name + `"/></div>`)
	if p1.Age == p2.Age {
		ret.Write(buildTextDiv(GREEN, strconv.Itoa(p2.Age), NONE))
	} else if p1.Age > p2.Age {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.Age), OVER))
	} else if p1.Age < p2.Age {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.Age), UNDER))
	}
	if p1.Club == p2.Club {
		ret.Write(buildTextDiv(GREEN, `<img src="`+p2.Club+`"/>`, NONE))
	} else if p1.ClubLeague == p2.ClubLeague {
		ret.Write(buildTextDiv(YELLOW, `<img src="`+p2.Club+`"/>`, NONE))
	} else {
		ret.Write(buildTextDiv(RED, `<img src="`+p2.Club+`"/>`, NONE))
	}
	if p1.NationalTeam == p2.NationalTeam {
		ret.Write(buildTextDiv(GREEN, `<img src="`+p2.NationalTeam+`"/>`, NONE))
	} else if getContinent(p1.NationalTeamCode) == getContinent(p2.NationalTeamCode) {
		ret.Write(buildTextDiv(YELLOW, `<img src="`+p2.NationalTeam+`"/>`, NONE))
	} else {
		ret.Write(buildTextDiv(RED, `<img src="`+p2.NationalTeam+`"/>`, NONE))
	}
	if p1.ShirtNumber == p2.ShirtNumber {
		ret.Write(buildTextDiv(GREEN, strconv.Itoa(p2.ShirtNumber), NONE))
	} else if p1.ShirtNumber > p2.ShirtNumber {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.ShirtNumber), OVER))
	} else if p1.ShirtNumber < p2.ShirtNumber {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.ShirtNumber), UNDER))
	}
	if p1.Position == p2.Position {
		ret.Write(buildTextDiv(GREEN, p2.Position, NONE))
	} else {
		ret.Write(buildTextDiv(RED, p2.Position, NONE))
	}
	if p1.L5 == p2.L5 {
		ret.Write(buildTextDiv(GREEN, strconv.Itoa(p2.L5), NONE))
	} else if p1.L5 > p2.L5 {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.L5), OVER))
	} else if p1.L5 < p2.L5 {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.L5), UNDER))
	}
	if p1.L15 == p2.L15 {
		ret.Write(buildTextDiv(GREEN, strconv.Itoa(p2.L15), NONE))
	} else if p1.L15 > p2.L15 {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.L15), OVER))
	} else if p1.L15 < p2.L15 {
		ret.Write(buildTextDiv(RED, strconv.Itoa(p2.L15), UNDER))
	}
	ret.WriteString("</div>")
	if winner {
		ret.WriteString(fmt.Sprintf(`
			<div class="winner" id="winner">
				<h2>Good Job ! You found <span>%s</span> in %d trys !
			</div>
		`, p2.Name, trys))
		numberOfFound++
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
					code
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
		ret.NationalTeamCode = strings.ToUpper(player.Football.Player.Country.Code)
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
	players = players[:n]
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(players), func(i, j int) { players[i], players[j] = players[j], players[i] })
	return players
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
						cardSupply {
							limited
						}
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
		if len(p.CardSupply) > 0 {
			ret = append(ret, playersub{Slug: p.Slug, Subscriptions: p.Subscriptions, DisplayName: p.DisplayName})
		}
	}
	return ret
}

func getContinent(code string) Continent {
	switch code {
	case "AF":
		return Asia
	case "AL":
		return Europe
	case "DZ":
		return Africa
	case "AS":
		return Oceania
	case "AD":
		return Europe
	case "AO":
		return Africa
	case "AI":
		return Americas
	case "AG":
		return Americas
	case "AR":
		return Americas
	case "AM":
		return Asia
	case "AW":
		return Americas
	case "AU":
		return Oceania
	case "AT":
		return Europe
	case "AZ":
		return Asia
	case "BS":
		return Americas
	case "BH":
		return Asia
	case "BD":
		return Asia
	case "BB":
		return Americas
	case "BY":
		return Europe
	case "BE":
		return Europe
	case "BZ":
		return Americas
	case "BJ":
		return Africa
	case "BM":
		return Americas
	case "BT":
		return Asia
	case "BO":
		return Americas
	case "BA":
		return Europe
	case "BW":
		return Africa
	case "BR":
		return Americas
	case "VG":
		return Americas
	case "BN":
		return Asia
	case "BG":
		return Europe
	case "BF":
		return Africa
	case "BI":
		return Africa
	case "KH":
		return Asia
	case "CM":
		return Africa
	case "CA":
		return Americas
	case "CV":
		return Africa
	case "KY":
		return Americas
	case "CF":
		return Africa
	case "TD":
		return Africa
	case "CL":
		return Americas
	case "CN":
		return Asia
	case "CX":
		return Asia
	case "CC":
		return Asia
	case "CO":
		return Americas
	case "KM":
		return Africa
	case "CG":
		return Africa
	case "CK":
		return Oceania
	case "CR":
		return Americas
	case "CI":
		return Africa
	case "HR":
		return Europe
	case "CU":
		return Americas
	case "CY":
		return Asia
	case "CZ":
		return Europe
	case "DK":
		return Europe
	case "DJ":
		return Africa
	case "DM":
		return Americas
	case "DO":
		return Americas
	case "EC":
		return Americas
	case "EG":
		return Africa
	case "SV":
		return Americas
	case "GQ":
		return Africa
	case "ER":
		return Africa
	case "EE":
		return Europe
	case "ET":
		return Africa
	case "FK":
		return Americas
	case "FO":
		return Europe
	case "FJ":
		return Oceania
	case "FI":
		return Europe
	case "FR":
		return Europe
	case "GF":
		return Americas
	case "PF":
		return Oceania
	case "GA":
		return Africa
	case "GM":
		return Africa
	case "GE":
		return Asia
	case "DE":
		return Europe
	case "GH":
		return Africa
	case "GI":
		return Europe
	case "GR":
		return Europe
	case "GL":
		return Americas
	case "GD":
		return Americas
	case "GP":
		return Americas
	case "GU":
		return Oceania
	case "GT":
		return Americas
	case "GN":
		return Africa
	case "GW":
		return Africa
	case "GY":
		return Americas
	case "HT":
		return Americas
	case "VA":
		return Europe
	case "HN":
		return Americas
	case "HU":
		return Europe
	case "IS":
		return Europe
	case "IN":
		return Asia
	case "ID":
		return Asia
	case "IR":
		return Asia
	case "IQ":
		return Asia
	case "IE":
		return Europe
	case "IL":
		return Asia
	case "IT":
		return Europe
	case "JM":
		return Americas
	case "JP":
		return Asia
	case "JO":
		return Asia
	case "KZ":
		return Asia
	case "KE":
		return Africa
	case "KI":
		return Oceania
	case "KP":
		return Asia
	case "KR":
		return Asia
	case "KW":
		return Asia
	case "KG":
		return Asia
	case "LA":
		return Asia
	case "LV":
		return Europe
	case "LB":
		return Asia
	case "LS":
		return Africa
	case "LR":
		return Africa
	case "LY":
		return Africa
	case "LI":
		return Europe
	case "LT":
		return Europe
	case "LU":
		return Europe
	case "MK":
		return Europe
	case "MG":
		return Africa
	case "MW":
		return Africa
	case "MY":
		return Asia
	case "MV":
		return Asia
	case "ML":
		return Africa
	case "MT":
		return Europe
	case "MH":
		return Oceania
	case "MQ":
		return Americas
	case "MR":
		return Africa
	case "MU":
		return Africa
	case "YT":
		return Africa
	case "MX":
		return Americas
	case "FM":
		return Oceania
	case "MD":
		return Europe
	case "MC":
		return Europe
	case "MN":
		return Asia
	case "MS":
		return Americas
	case "MA":
		return Africa
	case "MZ":
		return Africa
	case "NA":
		return Africa
	case "NR":
		return Oceania
	case "NP":
		return Asia
	case "NL":
		return Europe
	case "AN":
		return Americas
	case "NC":
		return Oceania
	case "NZ":
		return Oceania
	case "NI":
		return Americas
	case "NE":
		return Africa
	case "NG":
		return Africa
	case "NU":
		return Oceania
	case "NF":
		return Oceania
	case "MP":
		return Oceania
	case "NO":
		return Europe
	case "OM":
		return Asia
	case "PK":
		return Asia
	case "PW":
		return Oceania
	case "--":
		return Asia
	case "PA":
		return Americas
	case "PG":
		return Oceania
	case "PY":
		return Americas
	case "PE":
		return Americas
	case "PH":
		return Asia
	case "PN":
		return Oceania
	case "PL":
		return Europe
	case "PT":
		return Europe
	case "PR":
		return Americas
	case "QA":
		return Asia
	case "RE":
		return Africa
	case "RO":
		return Europe
	case "RU":
		return Asia
	case "RW":
		return Africa
	case "KN":
		return Americas
	case "LC":
		return Americas
	case "PM":
		return Americas
	case "VC":
		return Americas
	case "SM":
		return Europe
	case "ST":
		return Africa
	case "SA":
		return Asia
	case "SN":
		return Africa
	case "SC":
		return Africa
	case "SL":
		return Africa
	case "SG":
		return Asia
	case "SK":
		return Europe
	case "SI":
		return Europe
	case "SB":
		return Oceania
	case "SO":
		return Africa
	case "ZA":
		return Africa
	case "ES":
		return Europe
	case "LK":
		return Asia
	case "SD":
		return Africa
	case "SR":
		return Americas
	case "SJ":
		return Europe
	case "SZ":
		return Africa
	case "SE":
		return Europe
	case "CH":
		return Europe
	case "SY":
		return Asia
	case "TW":
		return Asia
	case "TJ":
		return Asia
	case "TZ":
		return Africa
	case "TH":
		return Asia
	case "TG":
		return Africa
	case "TK":
		return Oceania
	case "TO":
		return Oceania
	case "TT":
		return Americas
	case "TN":
		return Africa
	case "TR":
		return Asia
	case "TM":
		return Asia
	case "TC":
		return Americas
	case "TV":
		return Oceania
	case "UG":
		return Africa
	case "UA":
		return Europe
	case "AE":
		return Asia
	case "GB":
		return Europe
	case "GB-ENG":
		return Europe
	case "GB-WLS":
		return Europe
	case "GB-SCT":
		return Europe
	case "GB-NIR":
		return Europe
	case "US":
		return Americas
	case "UY":
		return Americas
	case "UZ":
		return Asia
	case "VU":
		return Oceania
	case "VE":
		return Americas
	case "VN":
		return Asia
	case "VI":
		return Americas
	case "WF":
		return Oceania
	case "EH":
		return Africa
	case "WS":
		return Oceania
	case "YE":
		return Asia
	case "ZR":
		return Africa
	case "ZM":
		return Africa
	case "ZW":
		return Africa
	}
	return Other
}
