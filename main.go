package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/integrii/flaggy"

	"github.com/chrisfentiman/rpc-claymore"
	"github.com/chrisfentiman/whattomine"
	cmc "github.com/coincircle/go-coinmarketcap"
	"gopkg.in/telegram-bot-api.v4"
)

var cfgPath string

func init() {
	flaggy.String(&cfgPath, "cp", "config_path", "The full path where the config.json file is located if not located in the same folder as Tele Monitor")
}

func main() {
	t, err := start()
	if err != nil {
		log.Panic(err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := t.bot.GetUpdatesChan(u)
	if err != nil {
		log.Panic(err)
	}

	for update := range updates {
		switch {
		case update.CallbackQuery != nil:
			cid := update.CallbackQuery.Message.Chat.ID
			mid := update.CallbackQuery.Message.MessageID
			switch {
			case strings.Contains(strings.ToLower(update.CallbackQuery.Data), "info"):
				s := strings.SplitAfter(update.CallbackQuery.Data, " ")
				s = append(s[:0], s[1:]...)
				t.sysQuery(cid, mid, s...)
			case strings.Contains(strings.ToLower(update.CallbackQuery.Data), "stats"):

			case strings.Contains(strings.ToLower(update.CallbackQuery.Data), "profit"):

			case strings.Contains(strings.ToLower(update.CallbackQuery.Data), "reboot"):
				s := strings.SplitAfter(update.CallbackQuery.Data, " ")
				s = append(s[:0], s[1:]...)
				t.rebootQuery(cid, mid, s...)
			case strings.Contains(strings.ToLower(update.CallbackQuery.Data), "restart"):
				s := strings.SplitAfter(update.CallbackQuery.Data, " ")
				s = append(s[:0], s[1:]...)
				t.restartQuery(cid, mid, s...)
			case strings.Contains(strings.ToLower(update.CallbackQuery.Data), "cancel"):
			}
		case update.Message != nil:
			cid := update.Message.Chat.ID
			mid := update.Message.MessageID
			if !t.isAuth(update.Message.From.UserName) {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unauthorized access, please contact system admin")
				msg.ReplyToMessageID = update.Message.MessageID
				if _, err := t.bot.Send(msg); err != nil {
					log.Println(err)
				}
				continue
			}

			switch {
			case strings.Contains(strings.ToLower(update.Message.Text), "info"):
				t.sysQuery(cid, mid)
			case strings.Contains(strings.ToLower(update.Message.Text), "stats"):

			case strings.Contains(strings.ToLower(update.Message.Text), "profit"):

			case strings.Contains(strings.ToLower(update.Message.Text), "reboot"):
				t.rebootQuery(cid, mid)
			case strings.Contains(strings.ToLower(update.Message.Text), "restart"):
				t.restartQuery(cid, mid)
			default:
				s := fmt.Sprintln(fmt.Sprintf("Sorry, I didn't understand your request: <i>\"%s\"</i> these are some requests you can make:", update.Message.Text))
				s += fmt.Sprintln("<b>info</b> <i>- Get critical info for all rigs or specifc rig.</i>")
				s += fmt.Sprintln("<b>stats</b> <i>- Get the statstics for all rigs or specific rig.</i>")
				s += fmt.Sprintln("<b>profit</b> <i>- Get the profit for all rigs or specific rig.</i>")
				s += fmt.Sprintln("<b>reboot</b> <i>- Reboot the system for all rigs or specific rig.</i>")
				s += fmt.Sprintln("<b>restart</b> <i>- Restart all rigs or a specific rig.</i>")
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, s)
				msg.ParseMode = "HTML"
				msg.ReplyToMessageID = update.Message.MessageID

				if _, err := t.bot.Send(msg); err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func (t *tele) isAuth(username string) bool {
	for _, u := range t.cfg.AuthUsers {
		if u == username {
			return true
		}
	}

	return false
}

type tele struct {
	bot      *tgbotapi.BotAPI
	rigfigs  map[string]rigfig
	rigs     map[string]rig
	prevRigs map[string]rig
	cfg      *botConfig
	db       *bolt.DB
}

type botConfig struct {
	Currency        string   `json:"currency"`
	CurrencySymbol  string   `json:"currency_symbol"`
	ElectricityCost float64  `json:"electricity_cost"`
	Poll            int64    `json:"miner_poll"`
	Token           string   `json:"token"`
	AuthUsers       []string `json:"auth_usernames"`
	NotifyChan      string   `json:"notification_channel"`
	Rigs            []struct {
		Name       string     `json:"name"`
		Addr       string     `json:"host"`
		Port       int        `json:"port"`
		Password string `json:"password"`
		IsDual     bool       `json:"is_dual"`
		CoinName   string     `json:"coin_name"`
		CoinTag    string     `json:"coin_tag"`
		Thresholds thresholds `json:"gpu_thresholds"`
	} `json:"clay"`
}

type thresholds struct {
	HashRate    int `json:"hashrate"`
	AltHashRate int `json:"althashrate"`
	Temperature int `json:"temp"`
	FanSpeed    int `json:"fan_speed"`
}

type rigfig struct {
	Host       claymore.Miner
	Name       string
	IsDual     bool
	CoinName   string
	CoinTag    string
	Thresholds thresholds
}

type gpus struct {
	HashRate    int
	AltHashRate int
	Temperature int
	FanSpeed    int
}

type rigHistory struct {
	Name              string
	LastSeen          time.Time
	MedianHashRate    int
	MedianAltHashRate int
	MedianTemperature int
	MedianFanSpeed    int
	LastGPUCount      int
}

type rig struct {
	Name       string
	CoinName   string
	CoinTag    string
	Status     string
	Version    string
	UpTime     int
	MainCrypto claymore.Crypto
	AltCrypto  claymore.Crypto
	MainPool   claymore.PoolInfo
	AltPool    claymore.PoolInfo
	GPUS       []gpus
}

func loadConfiguration() *botConfig {
	var config botConfig
	file := "config.json"
	if cfgPath != "" {
		file = cfgPath
	}

	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		fmt.Println(err.Error())
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&config)
	return &config
}

func start() (*tele, error) {
	cfg := loadConfiguration()
	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		log.Panic(err)
	}

	rigfig := rigSetup(cfg)

	db, err := bolt.Open("telem.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}

	t := &tele{
		bot:     bot,
		rigfigs: rigfig,
		cfg:     cfg,
		db:      db,
	}

	go t.poll()

	return t, nil
}

func rigSetup(cfg *botConfig) map[string]rigfig {
	rigs := make(map[string]rigfig, 0)
	for _, rig := range cfg.Rigs {
		rigs[rig.Name] = rigfig{
			Host:       claymore.New(fmt.Sprintf("%s:%v", rig.Addr, rig.Port), ""),
			Name:       rig.Name,
			CoinName:   rig.CoinName,
			CoinTag:    rig.CoinTag,
			IsDual:     rig.IsDual,
			Thresholds: rig.Thresholds,
		}
	}

	return rigs
}

func (t *tele) sysQuery(chatID int64, messageID int, names ...string) {
	switch {
	case len(t.rigs) == 1 && len(names) < 1:
		t.sysStatus(chatID, messageID)
	case len(names) >= 1:
		for _, name := range names {
			switch name {
			case "syswide":
				t.sysStatus(chatID, messageID)
			default:
				t.rigStatus(chatID, messageID, name)
			}
		}
	default:
		rows := make([]tgbotapi.InlineKeyboardButton, 0)
		for _, rig := range t.rigs {
			rows = append(rows, tgbotapi.NewInlineKeyboardButtonData(rig.Name, fmt.Sprintf("info %s", rig.Name)))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardButtonData("Entire System", fmt.Sprintf("info syswide")))
		msg := tgbotapi.NewMessage(chatID, "For which rig do you want info?")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows)
		if _, err := t.bot.Send(msg); err != nil {
			log.Println(err)
		}
	}
}

func (t *tele) rigStatus(chatID int64, messageID int, name string) {
	rig, ok := t.rigs[name]
	if !ok {
		s := fmt.Sprintf("Sorry, it seems %s does not exist, please check spelling or contact system admin.", rig.Name)
		s += fmt.Sprintln("Your request has been canceled.")
		msg := tgbotapi.NewMessage(chatID, s)
		if _, err := t.bot.Send(msg); err != nil {
			log.Println(err)
		}
		return
	}

	hashrate := float64(rig.MainCrypto.HashRate) / 1000

	s := fmt.Sprintln(fmt.Sprintf("Status: %s", rig.Status))
	s += fmt.Sprintln(fmt.Sprintf("Expected Daily Profit: %s", revenue(rig.CoinName, hashrate, t.cfg.Currency, t.cfg.CurrencySymbol)))
	s += fmt.Sprintln(fmt.Sprintf("Up Time: %v min", rig.UpTime))
	s += fmt.Sprintln(fmt.Sprintf("Total Shares: %v", rig.MainCrypto.Shares))
	s += fmt.Sprintln(fmt.Sprintf("Total Speed: %.3f Mh/s", hashrate))
	msg := tgbotapi.NewMessage(chatID, s)
	if _, err := t.bot.Send(msg); err != nil {
		log.Println(err)
	}
}

func revenue(CoinName string, HashRate float64, Currency string, CurrencySymbol string) string {
	c, err := wtm.GetCoin(CoinName)
	if err != nil {
		log.Println(err)
	}

	ticker, err := cmc.Ticker(&cmc.TickerOptions{
		Symbol:  c.Tag,
		Convert: Currency,
	})

	if err != nil {
		log.Fatal(err)
	}

	hash := HashRate * 1e6
	ratio := hash / float64(c.NetHash)
	blocksPerMin := 60.0 / float64(c.BlockTime)
	coinPerMin := blocksPerMin * c.BlockReward
	earnings := ratio * coinPerMin * 1440
	rev := ticker.Quotes[Currency].Price * earnings
	return fmt.Sprintf("%s%.2f", CurrencySymbol, rev)
}

func (t *tele) sysStatus(chatID int64, messageID int) {
	for _, rig := range t.rigs {
		t.rigStatus(chatID, messageID, rig.Name)
	}
}

func (t *tele) rebootQuery(chatID int64, messageID int, names ...string) {
	switch {
	case len(t.rigs) == 1 && len(names) < 1:
		for _, rig := range t.rigfigs {
			t.rebootConf(chatID, messageID, rig.Name)
		}
	case len(names) >= 1:
		for _, name := range names {
			switch name {
			case "syswide":
				t.rebootAll(chatID, messageID)
			default:
				t.rebootRig(chatID, messageID, name)
			}
		}
	default:
		rows := make([]tgbotapi.InlineKeyboardButton, 0)
		for _, rig := range t.rigs {
			rows = append(rows, tgbotapi.NewInlineKeyboardButtonData(rig.Name, fmt.Sprintf("reboot %s", rig.Name)))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardButtonData("Entire System", fmt.Sprintf("reboot syswide")))
		msg := tgbotapi.NewMessage(chatID, "For which rig do you want reboot?")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows)
		if _, err := t.bot.Send(msg); err != nil {
			log.Println(err)
		}
	}
}

func (t *tele) rebootAll(chatID int64, messageID int) {
	for _, rig := range t.rigfigs {
		t.rebootConf(chatID, messageID, rig.Name)
	}
}

func (t *tele) rebootConf(chatID int64, messageID int, name string) {
	rig, ok := t.rigfigs[name]
	if !ok {
		s := fmt.Sprintf("Sorry, it seems %s does not exist, please check spelling or contact system admin.", rig.Name)
		s += fmt.Sprintln("Your request to reboot rig has been canceled")
		msg := tgbotapi.NewMessage(chatID, s)
		if _, err := t.bot.Send(msg); err != nil {
			log.Println(err)
		}
		return
	}
	rows := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("Yes", fmt.Sprintf("reboot %s", rig.Name)),
		tgbotapi.NewInlineKeyboardButtonData("No", "cancel"),
	}
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Are you sure you want to reboot %s", rig.Name))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows)
	if _, err := t.bot.Send(msg); err != nil {
		log.Println(err)
	}
}

func (t *tele) rebootRig(chatID int64, messageID int, name string) {
	rig, ok := t.rigfigs[name]
	if !ok {
		s := fmt.Sprintf("Sorry, it seems %s does not exist, please check spelling or contact system admin.", rig.Name)
		s += fmt.Sprintln("Your request to reboot rig has been canceled")
		msg := tgbotapi.NewMessage(chatID, s)
		if _, err := t.bot.Send(msg); err != nil {
			log.Println(err)
		}
		return
	}

	if err := rig.Host.Reboot(); err != nil {
		s := fmt.Sprintf("ERROR: Rebooting rig %s has failed;", rig.Name)
		s += fmt.Sprintln(fmt.Sprintf("%s", err))
		msg := tgbotapi.NewMessage(chatID, s)
		if _, err := t.bot.Send(msg); err != nil {
			log.Println(err)
		}
	}
}

func (t *tele) restartQuery(chatID int64, messageID int, names ...string) {
	switch {
	case len(t.rigs) == 1 && len(names) < 1:
		for _, rig := range t.rigfigs {
			t.restartConf(chatID, messageID, rig.Name)
		}
	case len(names) >= 1:
		for _, name := range names {
			switch name {
			case "syswide":
				t.restartAll(chatID, messageID)
			default:
				t.restartRig(chatID, messageID, name)
			}
		}
	default:
		rows := make([]tgbotapi.InlineKeyboardButton, 0)
		for _, rig := range t.rigs {
			rows = append(rows, tgbotapi.NewInlineKeyboardButtonData(rig.Name, fmt.Sprintf("restart %s", rig.Name)))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardButtonData("Entire System", fmt.Sprintf("restart syswide")))
		msg := tgbotapi.NewMessage(chatID, "For which rig do you want restart?")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows)
		if _, err := t.bot.Send(msg); err != nil {
			log.Println(err)
		}

	}
}

func (t *tele) restartConf(chatID int64, messageID int, name string) {
	rig, ok := t.rigfigs[name]
	if !ok {
		s := fmt.Sprintf("Sorry, it seems %s does not exist, please check spelling or contact system admin.", rig.Name)
		s += fmt.Sprintln("Your request to restart rig has been canceled")
		msg := tgbotapi.NewMessage(chatID, s)
		if _, err := t.bot.Send(msg); err != nil {
			log.Println(err)
		}
		return
	}
	rows := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("Yes", fmt.Sprintf("restart %s", rig.Name)),
		tgbotapi.NewInlineKeyboardButtonData("No", "cancel"),
	}
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Are you sure you want to restart %s", rig.Name))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows)
	if _, err := t.bot.Send(msg); err != nil {
		log.Println(err)
	}
}

func (t *tele) restartAll(chatID int64, messageID int) {
	for _, rig := range t.rigfigs {
		t.restartConf(chatID, messageID, rig.Name)
	}
}

func (t *tele) restartRig(chatID int64, messageID int, name string) {
	rig, ok := t.rigfigs[name]
	if !ok {
		s := fmt.Sprintf("Sorry, it seems %s does not exist, please check spelling or contact system admin.", rig.Name)
		s += fmt.Sprintln("Your request to restart rig has been canceled.")
		msg := tgbotapi.NewMessage(chatID, s)
		if _, err := t.bot.Send(msg); err != nil {
			log.Println(err)
		}
		return
	}

	if err := rig.Host.Restart(); err != nil {
		s := fmt.Sprintf("ERROR: Restarting rig %s has failed;", rig.Name)
		s += fmt.Sprintln(fmt.Sprintf("%s", err))
		msg := tgbotapi.NewMessage(chatID, s)
		if _, err := t.bot.Send(msg); err != nil {
			log.Println(err)
		}
	}
}

func (t *tele) poll() {
	for {
		rigs := t.clayQuery()
		t.prevRigs = t.rigs
		t.rigs = rigs
		time.Sleep(time.Duration(t.cfg.Poll) * time.Second)
	}
}

func (t *tele) clayQuery() map[string]rig {
	rigs := make(map[string]rig, 0)

	for _, cfg := range t.rigfigs {
		status := "Online"
		r, err := cfg.Host.GetInfo()
		if err != nil {
			if rig, ok := t.rigs[cfg.Name]; ok {
				if rig.Status != "Offline" {
					s := fmt.Sprintf("ERROR: Rig %s not found %s", cfg.Name, err)
					msg := tgbotapi.NewMessageToChannel(t.cfg.NotifyChan, s)
					if _, err := t.bot.Send(msg); err != nil {
						log.Println(err)
					}

					rig.Status = "Offline"
					rigs[cfg.Name] = rig
				}
			} 
				log.Println(fmt.Sprintf("ERROR: Rig %s not found %s", cfg.Name, err))
			continue
		}

		if _, ok := t.rigs[cfg.Name]; !ok {
			s := fmt.Sprintf("Rig %s is now %s", cfg.Name, status)
			msg := tgbotapi.NewMessageToChannel(t.cfg.NotifyChan, s)
			if _, err := t.bot.Send(msg); err != nil {
				log.Println(err)
			}
		}


		if t.prevRigs != nil {
			was := len(t.prevRigs[cfg.Name].GPUS)
			cur := len(r.GPUS)
			if was > cur {
				s := fmt.Sprintf("ERROR: A GPU has gone down in rig %s, was %v now %v.", cfg.Name, was, cur)
				msg := tgbotapi.NewMessageToChannel(t.cfg.NotifyChan, s)
				if _, err := t.bot.Send(msg); err != nil {
					log.Println(err)
				}
				status = "Wounded"
			}
		}

		gps := make([]gpus, 0)
		for i, mine := range r.GPUS {
			if mine.IsStuck() {
				s := fmt.Sprintf("ERROR: GPU#%v in rig %s is stuck. 0Mh/s", i, cfg.Name)
				msg := tgbotapi.NewMessageToChannel(t.cfg.NotifyChan, s)
				if _, err := t.bot.Send(msg); err != nil {
					log.Println(err)
				}
				status = "Wounded"
				continue
			}

			gps = append(gps, gpus{
				HashRate:    mine.HashRate,
				AltHashRate: mine.AltHashRate,
				FanSpeed:    mine.FanSpeed,
				Temperature: mine.Temperature,
			})

			if mine.Temperature >= cfg.Thresholds.Temperature {
				s := fmt.Sprintf("TEMP WARNING: GPU#%v in rig %s has a fever, %v°C", i, cfg.Name, mine.Temperature)
				msg := tgbotapi.NewMessageToChannel(t.cfg.NotifyChan, s)
				if _, err := t.bot.Send(msg); err != nil {
					log.Println(err)
				}
				continue
			}

			// Minus five because the threshold is recommended to be the maximum and this creates a warning
			if mine.FanSpeed <= cfg.Thresholds.FanSpeed-5 {
				s := fmt.Sprintf("FAN WARNING: GPU#%v in rig %s has decreased its fan speed too low, %v%%", i, cfg.Name, mine.FanSpeed)
				msg := tgbotapi.NewMessageToChannel(t.cfg.NotifyChan, s)
				if _, err := t.bot.Send(msg); err != nil {
					log.Println(err)
				}
				continue
			}

			if mine.HashRate <= cfg.Thresholds.HashRate {
				s := fmt.Sprintf("HASHRATE WARNING: GPU#%v in rig %s has lost its hashrate, %v Mh/s", i, cfg.Name, mine.HashRate)
				msg := tgbotapi.NewMessageToChannel(t.cfg.NotifyChan, s)
				if _, err := t.bot.Send(msg); err != nil {
					log.Println(err)
				}
				status = "Wounded"
				continue
			}

			if cfg.IsDual {
				if mine.AltHashRate <= cfg.Thresholds.AltHashRate {
					s := fmt.Sprintf("ALT HASHRATE WARNING: GPU#%v in rig %s has lost its hashrater, %v Mh/s", i, cfg.Name, mine.AltHashRate)
					msg := tgbotapi.NewMessageToChannel(t.cfg.NotifyChan, s)
					if _, err := t.bot.Send(msg); err != nil {
						log.Println(err)
					}
					status = "Wounded"
					continue
				}
			}
		}

		rigs[cfg.Name] = rig{
			Status:     status,
			Name:       cfg.Name,
			CoinName:   cfg.CoinName,
			CoinTag:    cfg.CoinTag,
			Version:    r.Version,
			UpTime:     r.UpTime,
			MainCrypto: r.MainCrypto,
			MainPool:   r.MainPool,
			AltCrypto:  r.AltCrypto,
			AltPool:    r.AltPool,
			GPUS:       gps,
		}
	}

	return rigs
}
