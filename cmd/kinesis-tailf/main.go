package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"

	ktail "github.com/fujiwara/kinesis-tailf"
)

var (
	streamName = ""
	appendLF   = false
	region     = ""
)

func main() {
	shardIds := make([]string, 0)
	var shardId string
	flag.BoolVar(&appendLF, "lf", false, "append LF(\\n) to each record")
	flag.StringVar(&streamName, "stream", "", "stream name")
	flag.StringVar(&shardId, "shard-id", "", "shard id (, separated)")
	flag.StringVar(&region, "region", "", "region")
	flag.Parse()

	var sess *session.Session
	if region != "" {
		sess = session.New(
			&aws.Config{Region: aws.String(region)},
		)
	} else {
		sess = session.New()
	}

	k := kinesis.New(sess)
	sd, err := k.DescribeStream(&kinesis.DescribeStreamInput{
		StreamName: aws.String(streamName),
	})
	if err != nil {
		log.Fatal(err)
	}

	if shardId != "" {
		for _, s := range strings.Split(shardId, ",") {
			shardIds = append(shardIds, s)
		}
	} else {
		for _, s := range sd.StreamDescription.Shards {
			shardIds = append(shardIds, *s.ShardId)
		}
	}

	var wg sync.WaitGroup
	ch := make(chan []byte, 100)
	for _, id := range shardIds {
		wg.Add(1)
		go func(id string) {
			err := ktail.Iterate(k, streamName, id, ch)
			if err != nil {
				log.Println(err)
			}
			wg.Done()
		}(id)
	}
	wg.Add(1)
	go func() {
		writer(ch)
		wg.Done()
	}()
	wg.Wait()
}

func writer(ch chan []byte) {
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for {
		b := <-ch
		w.Write(b)
		if appendLF {
			w.Write(ktail.LF)
		}
		w.Flush()
	}
}
