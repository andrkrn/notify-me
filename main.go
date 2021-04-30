package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/andrkrn/notify-me/types"

	"gopkg.in/go-playground/webhooks.v5/github"
)

type Notify struct {
	Mention string // github username
	Channel string // slack channel
}

// Notify when project card moved to `ColumnId`
type ProjectNotify struct {
	ProjectId  int64
	ColumnId   int64
	ColumnName string
	Channel    string
}

var notifies = []Notify{
	{Mention: "@andrkrn", Channel: "#id-andrkrn"},
}

var project_notifies = []ProjectNotify{
	// TODO: get project id and column name with api
	{ColumnId: 13982492, ColumnName: "QA Test", Channel: "#id-andrkrn", ProjectId: 1},
}

func main() {
	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		ParseGithubPayload(rw, r)
	})

	if err := http.ListenAndServe(":3000", nil); err != nil {
		panic(err)
	}
}

func ParseGithubPayload(w http.ResponseWriter, r *http.Request) {
	webhook, err := github.New(github.Options.Secret(os.Getenv("SECRET")))

	if err != nil {
		panic(err)
	}

	event := r.Header.Get("X-GitHub-Event")
	is_project_card := github.Event(event) == github.ProjectCardEvent

	var payload interface{}

	if is_project_card {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil || len(body) == 0 {
			panic("error")
		}
		var pl types.ProjectCardPayloadPatch
		err = json.Unmarshal([]byte(body), &pl)

		if err != nil {
			panic(err)
		}

		payload = pl
	} else {
		pl, err := webhook.Parse(r, github.IssuesEvent, github.IssueCommentEvent)

		if err != nil {
			panic(err)
		}
		payload = pl
	}

	switch res := payload.(type) {
	case github.IssuesPayload:
		for _, notify := range notifies {
			if strings.Contains(res.Issue.Body, notify.Mention) {
				SendToSlack(res, notify)
			}
		}

	case github.IssueCommentPayload:
		for _, notify := range notifies {
			if strings.Contains(res.Comment.Body, notify.Mention) {
				SendToSlack(res, notify)
			}
		}

	case types.ProjectCardPayloadPatch:
		for _, project_notify := range project_notifies {
			if res.ProjectCard.ColumnID == project_notify.ColumnId {
				SendToSlack(res, project_notify)
			}
		}
	}
}

type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

type Attachment struct {
	Fallback string  `json:"fallback"`
	Pretext  string  `json:"pretext"`
	Color    string  `json:"color"`
	Fields   []Field `json:"fields"`
}

type Payload struct {
	Username    string       `json:"username"`
	Channel     string       `json:"channel"`
	Text        string       `json:"text"`
	IconEmoji   string       `json:"icon_emoji"`
	Attachments []Attachment `json:"attachments"`
}

func SendToSlack(payload interface{}, notify interface{}) {
	data := Payload{
		Username:  "Github Notification",
		IconEmoji: ":github:",
	}

	switch res := notify.(type) {
	case Notify:
		data.Channel = res.Channel
	case ProjectNotify:
		data.Channel = res.Channel
	}

	switch res := payload.(type) {
	case github.IssuesPayload:
		data.Text = fmt.Sprintf("<%s|%s>", res.Issue.HTMLURL, res.Issue.Title)
		data.Attachments = []Attachment{{
			Fallback: "You have been mentioned",
			Pretext:  "",
			Color:    "warning",
			Fields: []Field{{
				Title: "",
				Value: res.Issue.Body,
				Short: false,
			}},
		}}
	case github.IssueCommentPayload:
		data.Text = fmt.Sprintf("<%s|%s>", res.Comment.HTMLURL, res.Issue.Title)
		data.Attachments = []Attachment{{
			Fallback: "You have been mentioned",
			Pretext:  "",
			Color:    "warning",
			Fields: []Field{{
				Title: "",
				Value: res.Comment.Body,
				Short: false,
			}},
		}}
	case types.ProjectCardPayloadPatch:
		notifyRes := notify.(ProjectNotify)

		url := fmt.Sprintf("https://github.com/%s/projects/%v#card-%v", res.Repository.FullName, notifyRes.ProjectId, res.ProjectCard.ID)
		data.Text = fmt.Sprintf("<%s|This issue> just moved to %s", url, notifyRes.ColumnName)

	}

	payloadBytes, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", os.Getenv("SLACK_WEBHOOK_URL"), body)

	if err != nil {
		panic(err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
}
