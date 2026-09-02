package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type result struct{duration time.Duration;status int;err error}
func main(){var target,method,body,headerList string;var requests,concurrency int;var timeout time.Duration;flag.StringVar(&target,"url","http://127.0.0.1:8080/healthz","target URL");flag.StringVar(&method,"method","GET","HTTP method");flag.StringVar(&body,"body","","request body");flag.StringVar(&headerList,"headers","","semicolon-separated Header: value entries");flag.IntVar(&requests,"requests",10000,"total requests");flag.IntVar(&concurrency,"concurrency",100,"concurrent workers");flag.DurationVar(&timeout,"timeout",10*time.Second,"per-request timeout");flag.Parse();if requests<1||concurrency<1{fmt.Fprintln(os.Stderr,"requests and concurrency must be positive");os.Exit(2)};if concurrency>requests{concurrency=requests};headers:=parseHeaders(headerList);transport:=&http.Transport{MaxIdleConns:concurrency*2,MaxIdleConnsPerHost:concurrency,IdleConnTimeout:90*time.Second,ForceAttemptHTTP2:true};client:=&http.Client{Transport:transport,Timeout:timeout};jobs:=make(chan struct{},concurrency*2);results:=make(chan result,requests);var wg sync.WaitGroup;wg.Add(concurrency);started:=time.Now();for range concurrency{go func(){defer wg.Done();for range jobs{begin:=time.Now();req,err:=http.NewRequest(method,target,bytes.NewBufferString(body));if err==nil{for k,values:=range headers{for _,v:=range values{req.Header.Add(k,v)}};var resp *http.Response;resp,err=client.Do(req);if resp!=nil{_,_=io.Copy(io.Discard,resp.Body);_=resp.Body.Close();results<-result{duration:time.Since(begin),status:resp.StatusCode,err:err};continue}};results<-result{duration:time.Since(begin),err:err}}}()};go func(){for range requests{jobs<-struct{}{}};close(jobs);wg.Wait();close(results)}();latencies:=make([]time.Duration,0,requests);statuses:=map[int]int{};var errorsCount atomic.Int64;for r:=range results{latencies=append(latencies,r.duration);if r.err!=nil{errorsCount.Add(1)};if r.status!=0{statuses[r.status]++}};elapsed:=time.Since(started);sort.Slice(latencies,func(i,j int)bool{return latencies[i]<latencies[j]});fmt.Printf("requests=%d concurrency=%d elapsed=%s rps=%.1f errors=%d\n",requests,concurrency,elapsed.Round(time.Millisecond),float64(requests)/elapsed.Seconds(),errorsCount.Load());fmt.Printf("latency p50=%s p95=%s p99=%s max=%s\n",pct(latencies,.50),pct(latencies,.95),pct(latencies,.99),latencies[len(latencies)-1]);keys:=make([]int,0,len(statuses));for k:=range statuses{keys=append(keys,k)};sort.Ints(keys);for _,k:=range keys{fmt.Printf("status_%d=%d ",k,statuses[k])};fmt.Println();if errorsCount.Load()>0{os.Exit(1)}}
func parseHeaders(raw string)http.Header{h:=http.Header{};for _,part:=range strings.Split(raw,";"){part=strings.TrimSpace(part);if part==""{continue};key,value,ok:=strings.Cut(part,":");if ok{h.Add(strings.TrimSpace(key),strings.TrimSpace(value))}};return h}
func pct(values []time.Duration,p float64)time.Duration{if len(values)==0{return 0};i:=int(float64(len(values)-1)*p);return values[i]}
