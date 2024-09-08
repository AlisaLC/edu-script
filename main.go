package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type VahedRequest struct {
	Action string `json:"action"`
	Course string `json:"course"`
	Units  int32  `json:"units"`
}

type VahedResponse struct {
	Jobs              []*VahedJobResponse `json:"jobs"`
	RegisterationTime int64               `json:"registrationTime"`
	Time              int64               `json:"time"`
}

type VahedJobResponse struct {
	ID     string `json:"courseId"`
	Result string `json:"result"`
}

const EduUrl = "https://my.edu.sharif.edu/api/reg"
const AuthToken = "" // take from headers after login.

var mu sync.Mutex
var wg sync.WaitGroup
var vaheds = []*VahedRequest{
	{
		Action: "add",
		Course: "22034-2", // [CODE]-[GROUP]
		Units:  3,
	},
	{
		Action: "add",
		Course: "40441-1",
		Units:  3,
	},
} // fill with your courses in the above format.

func main() {
	client := &http.Client{}
	delay, err := findTimeDiff(client)
	if err != nil {
		fmt.Println(err)
		return
	}
	time.Sleep(delay)
	waitCount := 5
	if len(vaheds) > waitCount {
		waitCount = len(vaheds)
	}
	for {
		for _, vahed := range vaheds {
			wg.Add(1)
			go reqToEdu(client, vahed)
		}
		wg.Wait()
		time.Sleep(time.Duration(waitCount) * time.Second)
	}
}

func findTimeDiff(client *http.Client) (time.Duration, error) {
	req := initRequest(vaheds[0])
	time_start := time.Now()
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	time_end := time.Now()
	resp, err := parseResponse(res)
	if err != nil {
		return 0, err
	}
	server_time := time.Unix(resp.Time/1000, (resp.Time%1000)*1000000)
	time_diff := time_end.Sub(time_start)
	register_time := time.Unix(resp.RegisterationTime/1000, (resp.RegisterationTime%1000)*1000000)
	if server_time.After(register_time.Add(time.Hour)) {
		register_time = time.Date(server_time.Year(), server_time.Month(), server_time.Day(), 16, 0, 0, 0, server_time.Location())
	}
	delay := register_time.Sub(server_time) + time_diff + time.Millisecond*100 // 100 ms for net lag
	fmt.Println("Wait Time Until Start", delay)
	return delay, nil
}

func reqToEdu(client *http.Client, request *VahedRequest) {
	defer wg.Done()
	req := initRequest(request)
	mu.Lock() // remove if requests are slowed by the server. (currently it is.)
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	mu.Unlock()
	resp, err := parseResponse(res)
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(resp.Jobs) > 0 {
		for _, job := range resp.Jobs {
			if job.ID == request.Course {
				fmt.Println(job.ID, job.Result)
				break
			}
		}
	}
}

func initRequest(request *VahedRequest) *http.Request {
	payloadBuf := new(bytes.Buffer)
	json.NewEncoder(payloadBuf).Encode(request)
	req, _ := http.NewRequest("POST", EduUrl, payloadBuf)
	req.Header.Set("Authorization", AuthToken)
	req.Header.Set("Host", "my.edu.sharif.edu")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:104.0) Gecko/20100101 Firefox/104.0") // change to your own browser agent if you like.
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Referer", "https://my.edu.sharif.edu/courses/offered")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://my.edu.sharif.edu")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("TE", "trailers")
	return req
}

func parseResponse(res *http.Response) (*VahedResponse, error) {
	var Resp VahedResponse
	responseBuf := new(bytes.Buffer)
	if res.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(res.Body)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		io.Copy(responseBuf, reader)
	} else {
		io.Copy(responseBuf, res.Body)
	}
	defer res.Body.Close()
	err := json.Unmarshal(responseBuf.Bytes(), &Resp)
	if err != nil {
		if strings.Contains(responseBuf.String(), "REPEATED_REQUEST") {
			fmt.Println("REPEATED_REQUEST")
			time.Sleep(5 * time.Second)
		} else if strings.Contains(responseBuf.String(), "MAAREF_COURSES_LIMIT") {
			fmt.Println("MAAREF_COURSES_LIMIT")
			time.Sleep(5 * time.Second)
		} else if strings.Contains(responseBuf.String(), "CAPACITY_EXCEEDED") {
			fmt.Println("CAPACITY_EXCEEDED")
			time.Sleep(5 * time.Second)
		} else if strings.Contains(responseBuf.String(), "COURSE_NOT_FOUND") {
			fmt.Println("COURSE_NOT_FOUND")
			time.Sleep(5 * time.Second)
		}
		return nil, err
	}
	return &Resp, nil
}
