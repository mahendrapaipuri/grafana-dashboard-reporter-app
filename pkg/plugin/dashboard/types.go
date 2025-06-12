package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/chrome"
	"github.com/asanluis/grafana-dashboard-reporter-app/pkg/plugin/config"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// Dashboard represents a Grafana dashboard resource.
type Dashboard struct {
	logger         log.Logger
	conf           *config.Config
	httpClient     *http.Client
	chromeInstance chrome.Instance
	appURL         *url.URL
	appVersion     string
	jsContent      string
	model          *Model
	authHeader     http.Header
}

// RowOrPanel represents a container for Panels.
type RowOrPanel struct {
	Panel
	Collapsed bool    `json:"collapsed"`
	Panels    []Panel `json:"panels"`
}

// Model represents a Grafana JSON dashboard.
type Model struct {
	Meta struct {
		FolderUID   string `json:"folderUid"`
		FolderTitle string `json:"folderTitle"`
		FolderURL   string `json:"folderUrl"`
	} `json:"meta"`
	Dashboard struct {
		ID          int          `json:"id"`
		UID         string       `json:"uid"`
		Title       string       `json:"title"`
		Description string       `json:"description"`
		RowOrPanels []RowOrPanel `json:"panels"`
		Panels      []Panel
		Variables   url.Values
	} `json:"dashboard"`
}

// Data represents dashboard data that will be included in the report.
type Data struct {
	Title     string
	TimeRange TimeRange
	Variables string
	Panels    []Panel
}

type PanelType int

func (p PanelType) string() string {
	return [...]string{
		"singlestat",
		"text",
		"graph",
		"table",
	}[p]
}

const (
	SingleStat PanelType = iota
	Text
	Graph
	Table
)

// GridPos represents a Grafana dashboard panel position.
type GridPos struct {
	H float64 `json:"h"`
	W float64 `json:"w"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type PanelID string

func (i *PanelID) UnmarshalJSON(b []byte) error {
	var item interface{}
	if err := json.Unmarshal(b, &item); err != nil {
		return err
	}

	switch v := item.(type) {
	case int:
		*i = PanelID(strconv.Itoa(v))
	case float64:
		*i = PanelID(strconv.Itoa(int(v)))
	case string:
		*i = PanelID(v)
	}

	return nil
}

// Panel represents a Grafana dashboard panel.
type Panel struct {
	ID           string  `json:"-"`
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	GridPos      GridPos `json:"gridPos"`
	EncodedImage PanelImage
	CSVData      CSVData
}

func (p *Panel) String() string {
	return fmt.Sprintf("Panel ID: %s and Title: %s", p.ID, p.Title)
}

func (p *Panel) UnmarshalJSON(b []byte) error {
	type tmp Panel

	var s struct {
		tmp
		ID PanelID `json:"id"`
	}

	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}

	*p = Panel(s.tmp)
	p.ID = string(s.ID)

	return err
}

// IsSingleStat returns true if panel is of type SingleStat.
func (p *Panel) IsSingleStat() bool {
	return p.Is(SingleStat)
}

// IsPartialWidth If panel has width less than total allowable width.
func (p *Panel) IsPartialWidth() bool {
	return p.GridPos.W < 24
}

// Width returns the width of the panel.
func (p *Panel) Width() float64 {
	return float64(p.GridPos.W) * 0.04
}

// Height returns the height of the panel.
func (p *Panel) Height() float64 {
	return float64(p.GridPos.H) * 0.04
}

// Is returns true if panel is of type t.
func (p *Panel) Is(t PanelType) bool {
	return p.Type == t.string()
}

type PanelImage struct {
	Image    string
	MimeType string
}

func (p PanelImage) String() string {
	return fmt.Sprintf("data:%s;base64,%s", p.MimeType, p.Image)
}

// CSVData represents type of the CSV data.
type CSVData [][]string

type PanelTable struct {
	Title string
	Data  PanelTableData
}

type PanelTableData [][]string
