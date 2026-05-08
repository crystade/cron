package cron

import (
	"log"
	"testing"
	"time"
)

/*

	minute := field(fields[0], minutes)
	hour := field(fields[1], hours)
	dayOfMonth := field(fields[2], dom)
	month := field(fields[3], months)
	dayOfWeek := field(fields[4], dow)

*/

func TestA(t *testing.T) {
	parser, err := Parse("CRON_TZ=Asia/Saigon 0 0 1 * 0")
	if err != nil {
		log.Fatal(err)
	}
	nextTime := parser.Next(time.Now())
	log.Println(nextTime)
}
