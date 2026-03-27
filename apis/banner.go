package apis

import (
	"log"
	"strings"

	"github.com/fatih/color"
)

func StartBanner(baseURL string) {
	date := new(strings.Builder)
	log.New(date, "", log.LstdFlags).Print()

	bold := color.New(color.Bold).Add(color.FgGreen)
	bold.Printf("%s Server started at %s\n", strings.TrimSpace(date.String()), color.CyanString("%s", baseURL))

	regular := color.New()
	regular.Printf("├─ REST API:  %s\n", color.CyanString("%s/api/", baseURL))
	regular.Printf("└─ Dashboard: %s\n", color.CyanString("%s/_/", baseURL))
}
