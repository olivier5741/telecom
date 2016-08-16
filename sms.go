package main

import (
	"encoding/json"
	"fmt"
	"github.com/nu7hatch/gouuid"
	"github.com/shopspring/decimal"
	"time"
)

type Sms struct {
	Message  string
	Sender   *Sender
	Receiver *Receiver
	App      *App
	Pricing  *Pricing
}

type Receiver struct {
	PhoneNumber string
}

type Sender struct {
	PhoneNumber string
}

type App struct {
	ID         *uuid.UUID
	Order      *Order
	Fluid      bool
	TimeWindow *uuid.UUID
}

type Order struct {
	Number int
}

type WeekTimeWindow struct {
	TimeWindows []WeekDayTimeWindow
}

func (t WeekTimeWindow) Match(ti time.Time) bool {
	var daytime time.Duration = time.Duration(ti.Hour())*time.Hour + time.Duration(ti.Minute())*time.Minute + time.Duration(ti.Second())*time.Second
	for _, i := range t.TimeWindows {
		if ti.Weekday() == i.Weekday && daytime >= i.From && daytime <= i.To {
			return true
		}
	}
	return false
}

type TimeMatcher interface {
	Match(ti time.Time) bool
}

type WeekDayTimeWindow struct {
	Weekday time.Weekday
	From    time.Duration
	To      time.Duration
}

type TimeWindowDispatcher struct {
	timeWindows map[uuid.UUID]TimeMatcher
}

func (d *TimeWindowDispatcher) Dispatch(input <-chan Sms, match chan<- Sms, noMatch chan<- Sms) {
	go func() {
		for i := range input {
			if timeWindow, ok := d.timeWindows[i.AppID()]; ok {
				if timeWindow.Match(time.Now()) {
					match <- i
				} else {
					noMatch <- i
				}
			} else {
				match <- i
			}
		}
	}()
}

type Pricing struct {
	Price decimal.Decimal
}

type AddressGetter interface {
	Address() string
}

type Addresser interface {
	Address() string
	SetAddress(address string)
}

type Apper interface {
	AppID() uuid.UUID
}

func (s *Sms) AppID() uuid.UUID {
	return *s.App.ID
}

func (s *Sms) Price() decimal.Decimal {
	return s.Pricing.Price
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

type BlackListDispatcher struct {
	blackList map[string]bool
}

func (f *BlackListDispatcher) Dispatch(input <-chan Sms, whiteListed chan<- Sms, blackListed chan<- Sms) {
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

type AddressResolutionMutator struct {
	resolution map[uuid.UUID]string
}

// better if based on address and not sender
func (f *AddressResolutionMutator) Mutate(input <-chan Sms, output chan<- Sms) {
	go func() {
		for i := range input {
			for _, a := range i.Addresses() {
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

type PricingMutator struct {
	price decimal.Decimal
}

func (f *PricingMutator) Mutate(input <-chan Sms, output chan<- Sms) {
	go func() {
		for i := range input {
			i.Pricing = &Pricing{f.price}
			output <- i
		}
	}()
}

type PrepaidDispatcher struct {
	accounts map[uuid.UUID]decimal.Decimal
}

func (d *PrepaidDispatcher) Dispatch(input <-chan Sms, sufficientCredit chan<- Sms, insufficientCredit chan<- Sms) {
	go func() {
		for i := range input {
			if account, ok := d.accounts[i.AppID()]; ok {
				var inter = account.Sub(i.Price())
				if inter.Cmp(decimal.NewFromFloat(0)) < 0 {
					insufficientCredit <- i
				} else {
					fmt.Print("Credit : ")
					fmt.Println(inter)
					d.accounts[i.AppID()] = inter
					sufficientCredit <- i
				}
			} else {
				sufficientCredit <- i
			}
		}
	}()
}

type DeadEndConsumer struct {
	name string
}

func (f *DeadEndConsumer) Consume(input <-chan Sms) {
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
	sms := Sms{"Salut", &Sender{"8686"}, &Receiver{PhoneNumber: ""}, &App{ID: id}, nil}
	sms1 := Sms{"Salut", &Sender{PhoneNumber: "8686"}, &Receiver{PhoneNumber: "0476283272"}, nil, nil}

	input := make(chan Sms)
	input1 := make(chan Sms)
	output := make(chan Sms)
	blackListed := make(chan Sms)
	whiteListed := make(chan Sms)
	insufficientCredit := make(chan Sms)
	sufficientCredit := make(chan Sms)
	scheduled := make(chan Sms)
	execute := make(chan Sms)
	blackList := map[string]bool{"0476283272": true}
	resolution := map[uuid.UUID]string{*id: "0476283273"}
	accounts := map[uuid.UUID]decimal.Decimal{*id: decimal.NewFromFloat(1)}
	timeWindow := map[uuid.UUID]TimeMatcher{
		*id: WeekTimeWindow{
			[]WeekDayTimeWindow{
				{time.Monday, 9 * time.Hour, 18 * time.Hour}}}}

	filter_1 := AddressResolutionMutator{resolution}
	filter_1.Mutate(input, input1)

	filter0 := PricingMutator{decimal.NewFromFloat(0.5)}
	filter0.Mutate(input1, output)

	filter := BlackListDispatcher{blackList}
	filter.Dispatch(output, whiteListed, blackListed)

	filter1 := DeadEndConsumer{"Blacklisted"}
	filter1.Consume(blackListed)

	filter2 := PrepaidDispatcher{accounts}
	filter2.Dispatch(whiteListed, sufficientCredit, insufficientCredit)

	filter3 := DeadEndConsumer{"Insufficient credit"}
	filter3.Consume(insufficientCredit)

	filter4 := TimeWindowDispatcher{timeWindow}
	filter4.Dispatch(sufficientCredit, execute, scheduled)

	filter5 := DeadEndConsumer{"Scheduled"}
	filter5.Consume(scheduled)

	filter6 := DeadEndConsumer{"Execute"}
	filter6.Consume(execute)

	done := make(chan bool)
	go func() {
		input <- sms
		input <- sms
		input <- sms
		input <- sms1
		time.Sleep(time.Second)
		done <- true
	}()
	<-done
}
