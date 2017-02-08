package main

import(
	"log"
	"time"
	"flag"
	"net/url"

	"github.com/octago/sflags/gen/gflag"
)

type(
	Skate struct {
		Distribution    string        `desc:"Distribution URL (ex. http://registry.service.consul)"`
		TTL             time.Duration `desc:"Time after which the file is deleted (ex. 168h)"`
		Except          []string      `desc:"Ignore images (ex. redis,postgres)"`
		distribution    *Distribution
	}
)

func (skate *Skate) load() (*Distribution, error) {
	err := gflag.ParseToDef(skate)
	if err != nil {
		return nil, err
	}

	flag.Parse()

	url, err := url.Parse(skate.Distribution)
	if err != nil {
		return nil, err
	}

	return &Distribution{
		url: url,
		ttl: skate.TTL,
		except: skate.Except,
	}, nil
}

func main() {
	skate := &Skate{
		Distribution: "",
		TTL: time.Hour * 168,
		Except: []string{},
	}

	log.Println("Starting distribution cleanup tool")

	distribution, err := skate.load()
	if err != nil {
		log.Fatal("Configuration error:", err)
	}

	log.Println("Configuration is valid")
	log.Println("Checking distribution alive")

	err = distribution.check()
	if err != nil {
		log.Fatal("Distribution checking error:", err)
	}

	log.Println("Distribution is alive")
	log.Println("Cleaning up")

	err = distribution.cleanup()
	if err != nil {
		log.Fatal("Can't delete images:", err)
	}

}