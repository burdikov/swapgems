package main

import (
	"fmt"
	tb "github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	token, ok := os.LookupEnv("BOT_TOKEN")
	if !ok {
		log.Fatalln("fatal: where is the bot token?")
	}

	whSecret, ok := os.LookupEnv("BOT_WEBHOOK_SECRET")
	if !ok {
		log.Fatalln("fatal: where is the webhook secret?")
	}

	botDomain, ok := os.LookupEnv("BOT_DOMAIN")
	if !ok {
		log.Fatalln("fatal: where is the bot domain?")
	}

	webAppDomain, ok := os.LookupEnv("WEB_APP_DOMAIN")
	if !ok {
		log.Fatalln("fatal: where is the web app domain?")
	}

	bot, err := tb.NewBot(token, nil)

	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(bot.User.IsBot, bot.User.FirstName)

	dispatcher := ext.NewDispatcher(nil)
	updater := ext.NewUpdater(dispatcher, nil)

	dispatcher.AddHandler(handlers.NewCommand("start", func(b *tb.Bot, ctx *ext.Context) error {
		return start(b, ctx, webAppDomain)
	}))

	err = updater.AddWebhook(bot, "update", &ext.AddWebhookOpts{
		SecretToken: whSecret,
	})

	if err != nil {
		log.Fatalln(err)
	}

	botPath := "/bot/"
	err = updater.SetAllBotWebhooks(botDomain+botPath, &tb.SetWebhookOpts{
		AllowedUpdates:     nil,
		DropPendingUpdates: false,
		SecretToken:        whSecret,
	})

	if err != nil {
		log.Fatalln(err)
	}

	dispatcher.AddHandler(handlers.InlineQuery{
		Filter:   nil,
		Response: handleInlineQuery(webAppDomain),
	})

	log.Printf("%s has been started...\n", bot.User.Username)

	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const (
		letterIdxBits = 6                    // 6 bits to represent a letter index
		letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
		letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
	)
	var src = rand.NewSource(time.Now().UnixNano())
	var getKey = func(n int) string {
		sb := strings.Builder{}
		sb.Grow(n)
		// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
		for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
			if remain == 0 {
				cache, remain = src.Int63(), letterIdxMax
			}
			if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
				sb.WriteByte(letterBytes[idx])
				i--
			}
			cache >>= letterIdxBits
			remain--
		}

		return sb.String()
	}

	var formMessage = func(form url.Values) string {
		builder := strings.Builder{}

		builder.Write([]byte("*"))
		switch form["buy-or-sell"][0] {
		case "buy":
			builder.Write([]byte("Куплю "))
		case "sell":
			builder.Write([]byte("Продам "))
		}

		builder.WriteString(form["our-sum"][0] + " " + form["our-curr"][0] + "*\n")

		_, isCb := form["cb"]

		builder.Write([]byte("*"))
		switch form["buy-or-sell"][0] {
		case "buy":
			builder.Write([]byte("Продам "))
		case "sell":
			builder.Write([]byte("Куплю "))
		}

		if isCb {
			builder.Write([]byte(form["their-curr"][0] + " по ЦБ*"))
		} else {
			switch form["sum-or-rate"][0] {
			case "sum":
				builder.Write([]byte(form["their-sum"][0] + " " + form["their-curr"][0] + "*"))
			case "rate":
				builder.Write([]byte(form["their-curr"][0] + " по курсу " + form["rate"][0] + "*"))
			}
		}

		builder.Write([]byte("\n"))
		eu := strings.Join(form["eu-methods"], ", ")
		if len(form["eu-methods-str"][0]) > 0 {
			eu += ", " + form["eu-methods-str"][0]
		}

		if len(eu) > 0 {
			builder.Write([]byte("eu: " + eu + "\n"))
		}

		ru := strings.Join(form["ru-methods"], ", ")
		if len(form["ru-methods-str"][0]) > 0 {
			ru += ", " + form["ru-methods-str"][0]
		}

		if len(ru) > 0 {
			builder.Write([]byte("ru: " + ru + "\n"))
		}

		if len(form["location"][0]) > 0 {
			builder.Write([]byte("Наличные: " + form["location"][0] + "\n"))
		}

		if len(form["comment"]) > 0 {
			builder.WriteString(form["comment"][0] + "\n")
		}

		return builder.String()
	}

	mux := http.NewServeMux()
	mux.HandleFunc(botPath, updater.GetHandlerFunc(botPath))
	mux.HandleFunc("/bot/form", func(writer http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		log.Println(r.Form)

		key := getKey(5)
		database.Store(key, formMessage(r.Form)) // TODO you can send key right away and form msg in the bg

		writer.Header().Set("Access-Control-Allow-Origin", webAppDomain)
		_, _ = writer.Write([]byte(key))
	})

	server := http.Server{
		Handler: mux,
		Addr:    "0.0.0.0:8080",
	}

	if err := server.ListenAndServe(); err != nil {
		panic("failed to listen and serve: " + err.Error())
	}
}

var database = sync.Map{}

func answerEmpty(bot *tb.Bot, ctx *ext.Context, webAppUrl string) error {
	_, err := bot.AnswerInlineQuery(
		ctx.InlineQuery.Id,
		nil,
		&tb.AnswerInlineQueryOpts{
			Button: &tb.InlineQueryResultsButton{
				Text:   "Open form",
				WebApp: &tb.WebAppInfo{Url: webAppUrl},
			},
		},
	)

	return err
}

func handleInlineQuery(webAppUrl string) func(*tb.Bot, *ext.Context) error {
	return func(bot *tb.Bot, ctx *ext.Context) error {
		offer, ok := database.LoadAndDelete(ctx.InlineQuery.Query)

		if ctx.InlineQuery.Query == "" || !ok {
			return answerEmpty(bot, ctx, webAppUrl)
		}

		_, err := bot.AnswerInlineQuery(
			ctx.InlineQuery.Id,
			[]tb.InlineQueryResult{
				tb.InlineQueryResultContact{
					Id:          "1",
					PhoneNumber: "Жми сюда",
					FirstName:   "Опубликовать моё предложение",
					InputMessageContent: tb.InputTextMessageContent{
						MessageText:        fmt.Sprintf("%v", offer),
						ParseMode:          "markdown",
						Entities:           nil,
						LinkPreviewOptions: nil,
					},
				},
			}, nil,
		)

		return err
	}
}

// start introduces the bot.
func start(b *tb.Bot, ctx *ext.Context, webappURL string) error {
	_, err := ctx.EffectiveMessage.Reply(
		b,
		fmt.Sprintf(
			"Hello, I'm @%s.\n",
			b.User.Username,
		),
		&tb.SendMessageOpts{
			ParseMode: "HTML",
			ReplyMarkup: tb.InlineKeyboardMarkup{
				InlineKeyboard: [][]tb.InlineKeyboardButton{{
					{Text: "Press me", WebApp: &tb.WebAppInfo{Url: webappURL}},
				}},
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to send start message: %w", err)
	}
	return nil
}
