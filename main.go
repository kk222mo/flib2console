package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const BOOKS_PER_PAGE = 20

var start_text string = `
 _______  ___      ___   _______  __   __  _______  _______  _______
|       ||   |    |   | |  _    ||  | |  ||       ||       ||   _   |
|    ___||   |    |   | | |_|   ||  | |  ||  _____||_     _||  |_|  |
|   |___ |   |    |   | |       ||  |_|  || |_____   |   |  |       |
|    ___||   |___ |   | |  _   | |       ||_____  |  |   |  |       |
|   |    |       ||   | | |_|   ||       | _____| |  |   |  |   _   |
|___|    |_______||___| |_______||_______||_______|  |___|  |__| |__|
`

type Book struct {
	Link   string
	Author string
	Title  string
}

func min(a, b int) int {
	return int(math.Min(float64(a), float64(b)))
}

func max(a, b int) int {
	return int(math.Max(float64(a), float64(b)))
}

func PrintBooks(books []Book, pos int) {
	for i := pos; i < min(pos+BOOKS_PER_PAGE, len(books)); i++ {
		b := books[i]
		if b.Author != "" {
			b.Author = b.Author[:len(b.Author)-1]
			fmt.Printf("\033[31;1m%v\033[0m %v(%v)\n", b.Link, b.Title, b.Author)
		} else {
			fmt.Printf("\033[31;1m%v\033[0m %v\n", b.Link, b.Title)
		}
	}
	fmt.Printf("\033[37;1mРезультаты: %v-%v из %v\n(/next - следующая страница, /prev - предыдущая)\n\033[0m", pos, min(pos+BOOKS_PER_PAGE, len(books)), len(books))
}

func DownloadBook(b Book, client *http.Client) {
	fmt.Println("Начинаем скачивание...")
	req, err := http.NewRequest("GET", "http://flibusta.is/b/"+b.Link+"/fb2", nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/42.0.2311.135 Safari/537.36 Edge/12.246")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("\033[31;1mError downloading " + b.Link + ": " + err.Error() + "\033[0m")
		return
	}
	defer resp.Body.Close()
	pl := strings.Split(b.Link, "/")
	name := pl[len(pl)-1]
	out, err := os.Create(name + ".zip")
	if err != nil {
		fmt.Println("\033[31;1mError downloading " + b.Link + ": " + err.Error() + "\033[0m")
		return
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Println("\033[31;1mError downloading " + b.Link + ": " + err.Error() + "\033[0m")
		return
	}
	fmt.Println("\033[32;1mУспешно\033[0m")
}

var commands_list string = `
1) /search <name>      найти книгу
2) /download <bid>     скачать книгу
3) /exit               выйти
`

var TorProxy string = "socks5://127.0.0.1:9050"

func SearchForBook(query string, client *http.Client) []Book {
	books := make([]Book, 0)
	for page := 0; page < 3; page++ {
		tb := Book{}
		req, err := http.NewRequest("GET", "http://flibusta.is/booksearch?ask="+query+"&page="+strconv.Itoa(page), nil)
		if err != nil {
			fmt.Println("\033[31;1mError suring search: " + err.Error() + "\033[0m")
			return nil
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/42.0.2311.135 Safari/537.36 Edge/12.246")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("\033[31;1mError suring search: " + err.Error() + "\033[0m")
			return nil
		}
		re_b := regexp.MustCompile(`/b/[0-9]+`)
		re_a := regexp.MustCompile(`/a/[0-9]+`)
		var f func(*html.Node, bool)
		f = func(n *html.Node, in bool) {
			if n.Data == "li" && !in {
				in = true
			} else if n.Data == "a" && in {
				for _, attr := range n.Attr {
					if attr.Key == "href" && re_a.Match([]byte(attr.Val)) {
						tb.Author += n.FirstChild.Data + ","
					} else if attr.Key == "href" && re_b.Match([]byte(attr.Val)) {
						title := ""
						for tc := n.FirstChild; tc != nil; tc = tc.NextSibling {
							if tc.Data == "b" || tc.Data == "span" {
								title += tc.FirstChild.Data
							} else {
								title += tc.Data
							}
						}
						if tb.Title != "" {
							books = append(books, tb)
						}
						tb = Book{}
						tb.Title = title
						lp := strings.Split(attr.Val, "/")
						tb.Link = lp[len(lp)-1]
					}
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c, in)
			}
		}
		if tb.Title != "" {
			books = append(books, tb)
		}
		doc, err := html.Parse(resp.Body)
		if err != nil {
			panic(err)
		}
		f(doc, false)
	}
	return books
}

func main() {

	torProxyUrl, _ := url.Parse(TorProxy)
	transport := &http.Transport{Proxy: http.ProxyURL(torProxyUrl)}
	client := &http.Client{Transport: transport, Timeout: time.Second * 10}
	fmt.Printf("\033[37;1m")
	fmt.Println(start_text)
	fmt.Println("\033[0m")
	pos := 0
	reader := bufio.NewReader(os.Stdin)
	books := make([]Book, 0)
	for {
		fmt.Println("\n\n" + commands_list)
		fmt.Printf("Команда: ")
		text, _ := reader.ReadString('\n')
		text = strings.Trim(text, "\n")
		params := strings.Split(text, " ")
		if params[0] == "/search" {
			if len(params) < 2 {
				fmt.Println("Неверная команда. Пример: /search Отцы и дети")
				continue
			}
			pos = 0
			name := strings.Join(params[1:], "+")
			books = SearchForBook(name, client)
			if books != nil {
				fmt.Println("\033[37;1mРезультаты поиска:\033[0m")
				PrintBooks(books, pos)
			}
		} else if params[0] == "/next" {
			pos += BOOKS_PER_PAGE
			PrintBooks(books, pos)
		} else if params[0] == "/prev" {
			pos = max(0, pos-BOOKS_PER_PAGE)
			PrintBooks(books, pos)
		} else if params[0] == "/download" {
			link := params[1]
			for _, b := range books {
				if b.Link == link {
					DownloadBook(b, client)
					break
				}
			}
		} else if params[0] == "/exit" {
			os.Exit(0)
		}
	}
}
