package main

import (
	"fmt"

	"github.com/mrd0ll4r/winminer"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.DebugLevel)

	fmt.Println("connecting...")
	client, err := winminer.NewAPIClient("you@example.com", "password", true)
	if err != nil {
		panic(err)
	}
	fmt.Println("connected.")

	stats, err := client.GetStats()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", stats)

	machines, err := client.GetMachines()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", machines)

	withdrawData, err := client.GetWithdrawData()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", withdrawData)

	withdrawHistory, err := client.GetWithdrawHistory()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", withdrawHistory)

	ltc, err := withdrawHistory.Transactions[0].ParseDataAsLitecoinTransaction()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", ltc)

	ws, err := client.ConnectWebsocket()
	if err != nil {
		panic(err)
	}

	state := winminer.NewLiveState()

	for i := 0; i < 10; i++ {
		messages, err := ws.ReadNextInterestingMessages()
		if err != nil {
			panic(err)
		}

		for _, msg := range messages.Messages {
			fmt.Printf("%+v\n", msg)
			switch msg.Method {
			case winminer.MethodSetSystemInfo:
				sysInf, err := winminer.ParseSystemInfoMessage(msg)
				if err != nil {
					panic(err)
				}

				state.SetSystemInfo(sysInf)
			case winminer.MethodStatusChanged:
				status, err := winminer.ParseStatusChangedMessage(msg)
				if err != nil {
					panic(err)
				}
				err = state.UpdateStatus(*status)
				if err != nil {
					panic(err)
				}
			}
		}
	}
}
