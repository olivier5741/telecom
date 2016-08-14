package main

import (
	"encoding/json"
	"fmt"
	"github.com/nu7hatch/gouuid"
	"math/big"
	"time"
)

type Sms struct {
	Message  string
	Sender   *Sender
	Receiver *Receiver
	App      *App
	Pricing  Pricing
}

type Receiver struct {
	PhoneNumber string
}

type Sender struct {
	PhoneNumber string
}

type App struct {
	ID *uuid.UUID
}

type Pricing *big.Rat

type AddressGetter interface {
	Address() string
}

type Addresser interface {
	Address() string
	SetAddress(address string)
}

func (s Receiver) Address() string {
	return s.PhoneNumber
}

func (s *Receiver) SetAddress(address string) {
	s.PhoneNumber = address
}

func (s *Sms) Addresses() []Addresser {
	return []Addresser{s.Receiver}
}

type BlackListFilter struct {
	blackList map[string]bool
}

func (f *BlackListFilter) Filter(input <-chan Sms, whiteListed chan<- Sms, blackListed chan<- Sms) {
	go func() {
		for i := range input {
			for _, a := range i.Addresses() {
				if _, notOk := f.blackList[a.Address()]; notOk {
					blackListed <- i
				} else {
					whiteListed <- i
				}
			}
		}
	}()
}

type AddressResolutionFilter struct {
	resolution map[uuid.UUID]string
}

// better if based on address and not sender
func (f *AddressResolutionFilter) Filter(input <-chan Sms, output chan<- Sms) {
	go func() {
		for i := range input {
			for _,a := range i.Addresses() {
				if a.Address() == "" && i.App.ID != nil {
					if address, ok := f.resolution[*i.App.ID]; ok {
						a.SetAddress(address)
					}
				}				
			}
			output <- i
		}
	}()
}

type PricingFilter struct {
	price *big.Rat
}

func (f *PricingFilter) Filter(input <-chan Sms, output chan<- Sms) {
	go func() {
		for i := range input {
			i.Pricing = f.price
			output <- i
		}
	}()
}

type DeadEndFilter struct {
	name string
}

func (f *DeadEndFilter) Filter(input <-chan Sms) {
	go func() {
		for i := range input {
			fmt.Print(f.name + " : ")
			m, _ := json.Marshal(i)
			fmt.Println(string(m))
		}
	}()
}

func main() {

	id, _ := uuid.NewV5(uuid.NamespaceURL, []byte("809R3NF2"))
	sms := Sms{"Salut", &Sender{"8686"}, &Receiver{PhoneNumber: ""}, &App{id}, nil}
	sms1 := Sms{"Salut", &Sender{PhoneNumber: "8686"}, &Receiver{PhoneNumber: "0476283272"}, nil, nil}

	input := make(chan Sms)
	input1 := make(chan Sms)
	output := make(chan Sms)
	whiteListed := make(chan Sms)
	blackListed := make(chan Sms)
	blackList := map[string]bool{"0476283272": true}
	resolution := map[uuid.UUID]string{*id: "0476283273"}

	filter_1 := AddressResolutionFilter{resolution}
	filter_1.Filter(input, input1)

	filter0 := PricingFilter{new(big.Rat).SetFloat64(0.05)}
	filter0.Filter(input1, output)

	filter := BlackListFilter{blackList}
	filter.Filter(output, whiteListed, blackListed)

	filter1 := DeadEndFilter{"Blacklisted"}
	filter1.Filter(blackListed)
	filter2 := DeadEndFilter{"Whitelisted"}
	filter2.Filter(whiteListed)

	done := make(chan bool)
	go func() {
		input <- sms
		input <- sms
		input <- sms1
		time.Sleep(time.Second)
		done <- true
	}()
	<-done
}
