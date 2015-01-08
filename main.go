/*

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>.

*/

package main

import (
	"flag"
	"fmt"
	"github.com/golang/groupcache/lru"
	"github.com/kz26/m3u8"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const VERSION = "0.0.1"

var USER_AGENT string

var client = &http.Client{}

var VERBOSE bool

func doRequest(c *http.Client, req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", USER_AGENT)
	resp, err := c.Do(req)
	return resp, err
}

type Download struct {
	URI           string
	totalDuration time.Duration
	filename      string
}

func downloadSegment(fn string, dlc chan *Download, recTime time.Duration) {
	for v := range dlc {
		req, err := http.NewRequest("GET", v.URI, nil)
		if err != nil {
			log.Fatal(err)
		}
		resp, err := doRequest(client, req)
		if err != nil {
			log.Print(err)
			continue
		}
		if resp.StatusCode != 200 {
			log.Printf("ERROR: HTTP %v at position %v for %v\n", resp.StatusCode, v.totalDuration, v.URI)
			continue
		}
		if fn != "" {
			var out *os.File
			filename := fn + string(os.PathSeparator) + v.filename
			log.Print(filename)
			_, err := os.Stat(filename)
			if err != nil {
				out, err = os.Create(filename)
				if err != nil {
					log.Fatal(err)
				}
			} else {
				out, err = os.OpenFile(filename, os.O_RDWR|os.O_APPEND, 0660)
			}

			defer out.Close()
			_, err = io.Copy(out, resp.Body)
			if err != nil {
				log.Fatal(err)
			}
		}

		resp.Body.Close()

		if VERBOSE {
			log.Printf("Downloaded %v\n", v.URI)
			if recTime != 0 {
				log.Printf("Recorded %v of %v\n", v.totalDuration, recTime)
			} else {
				log.Printf("Recorded %v\n", v.totalDuration)
			}
		}
	}
}

func getPlaylist(urlStr string, recTime time.Duration, useLocalTime bool, dlc chan *Download, closeWhenFinished bool, filename string) {
	startTime := time.Now()
	var recDuration time.Duration = 0
	cache := lru.New(1024)
	playlistUrl, err := url.Parse(urlStr)
	if err != nil {
		log.Fatal(err)
	}
	for {
		req, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			log.Fatal(err)
		}
		resp, err := doRequest(client, req)
		if err != nil {
			log.Print(err)
			time.Sleep(time.Duration(3) * time.Second)
		}
		playlist, listType, err := m3u8.DecodeFrom(resp.Body, false)
		if err != nil {
			log.Fatal(err)
		}
		resp.Body.Close()

		if listType == m3u8.MASTER {
			baseURL := urlStr[0 : strings.LastIndex(urlStr, "/")+1]

			if VERBOSE {
				log.Printf("Master playlist detected. Base URL: %s\n", baseURL)
			}

			mpl := playlist.(*m3u8.MasterPlaylist)
			var wg sync.WaitGroup
			for _, v := range mpl.Variants {
				playlistUrl := baseURL + v.URI
				filename := safeFilename(v.URI)

				if VERBOSE {
					log.Printf("Delegating playlist: %s", playlistUrl)
				}

				wg.Add(1)
				go func() {
					getPlaylist(playlistUrl, recTime, useLocalTime, dlc, false, filename)
					wg.Done()
				}()
			}

			if closeWhenFinished {
				wg.Wait()
				close(dlc)
			}
			return
		}

		if listType == m3u8.MEDIA {
			mpl := playlist.(*m3u8.MediaPlaylist)
			for _, v := range mpl.Segments {
				if v != nil {
					var msURI string
					if strings.HasPrefix(v.URI, "http") {
						msURI, err = url.QueryUnescape(v.URI)
						if err != nil {
							log.Fatal(err)
						}
					} else {
						msUrl, err := playlistUrl.Parse(v.URI)
						if err != nil {
							log.Print(err)
							continue
						}
						msURI, err = url.QueryUnescape(msUrl.String())
						if err != nil {
							log.Fatal(err)
						}
					}
					_, hit := cache.Get(msURI)
					if !hit {
						cache.Add(msURI, nil)
						if useLocalTime {
							recDuration = time.Now().Sub(startTime)
						} else {
							recDuration += time.Duration(int64(v.Duration * 1000000000))
						}
						dlc <- &Download{msURI, recDuration, filename}
					}
					if recTime != 0 && recDuration != 0 && recDuration >= recTime {
						if closeWhenFinished {
							close(dlc)
						}
						return
					}
				}
			}
			if mpl.Closed {
				if closeWhenFinished {
					close(dlc)
				}
				return
			} else {
				time.Sleep(time.Duration(int64(mpl.TargetDuration * 1000000000)))
			}
		} else {
			log.Fatal("Not a valid media playlist")
		}
	}
}

func safeFilename(filename string) string {
	return strings.Replace(filename, string(os.PathSeparator), "_", -1)
}

func main() {

	duration := flag.Duration("t", time.Duration(0), "Recording duration (0 == infinite)")
	useLocalTime := flag.Bool("l", false, "Use local time to track duration instead of supplied metadata")
	destination := flag.String("d", "", "Download destination (directory).")
	flag.StringVar(&USER_AGENT, "ua", fmt.Sprintf("hlsvalidator/%v", VERSION), "User-Agent for HTTP client")
	flag.BoolVar(&VERBOSE, "v", false, "Verbose output")
	flag.Parse()

	os.Stderr.Write([]byte(fmt.Sprintf("hlsvalidator %v - HTTP Live Streaming (HLS) validator\n", VERSION)))
	os.Stderr.Write([]byte("Copyright (C) 2015 Erik Wallentinsen (The Capitals)\n"))
	os.Stderr.Write([]byte("Original code by Kevin Zhang. Licensed for use GPL v3.\n\n"))

	if flag.NArg() != 1 {
		os.Stderr.Write([]byte("Usage: hlsvalidator [-l=bool (localtime)] [-v=bool (verbose output)] [-t duration] [-ua user-agent] [-d destination] hls-url\n"))
		flag.PrintDefaults()
		os.Exit(2)
	}

	if *destination != "" {
		src, err := os.Stat(*destination)
		if err != nil {
			log.Fatalf("Destination folder does not exist: %s", *destination)
		}

		// check if the source is indeed a directory or not
		if !src.IsDir() {
			log.Fatalf("Destination is not a directory: %s", *destination)
		}
	}

	if !strings.HasPrefix(flag.Arg(0), "http") {
		log.Fatal("Media playlist url must begin with http/https")
	}

	msChan := make(chan *Download, 1024)
	go getPlaylist(flag.Arg(0), *duration, *useLocalTime, msChan, true, "single")
	downloadSegment(*destination, msChan, *duration)
}
