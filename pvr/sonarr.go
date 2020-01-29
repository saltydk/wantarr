package pvr

import (
	"fmt"
	"github.com/imroc/req"
	"github.com/l3uddz/wantarr/config"
	"github.com/l3uddz/wantarr/logger"
	"github.com/l3uddz/wantarr/utils/web"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

/* Structs */

type Sonarr struct {
	cfg        *config.Pvr
	log        *logrus.Entry
	apiUrl     string
	reqHeaders req.Header
	version    int
}

type SonarrQueue struct {
	Size int `json:"totalRecords"`
}

type SonarrEpisode struct {
	Title         string
	Id            int
	SeasonNumber  int
	EpisodeNumber int
	AirDateUtc    time.Time
	Monitored     bool
}

type SonarrWanted struct {
	Page          int
	PageSize      int
	SortKey       string
	SortDirection string
	TotalRecords  int
	Records       []SonarrEpisode
}

type SonarrSystemStatus struct {
	Version string
}

/* Initializer */

func NewSonarr(name string, c *config.Pvr) *Sonarr {
	// set api url
	apiUrl := ""
	if strings.Contains(c.URL, "/api") {
		apiUrl = c.URL
	} else {
		apiUrl = web.JoinURL(c.URL, "/api/v3")
	}

	// set headers
	reqHeaders := req.Header{
		"X-Api-Key": c.ApiKey,
	}

	return &Sonarr{
		cfg:        c,
		log:        logger.GetLogger(name),
		apiUrl:     apiUrl,
		reqHeaders: reqHeaders,
	}
}

/* Private */

func (p *Sonarr) getSystemStatus() (*SonarrSystemStatus, error) {
	// send request
	resp, err := web.GetResponse(web.GET, web.JoinURL(p.apiUrl, "/system/status"), 15, p.reqHeaders)
	if err != nil {
		return nil, errors.New("failed retrieving system status api response from sonarr")
	}
	defer resp.Response().Body.Close()

	// validate response
	if resp.Response().StatusCode != 200 {
		return nil, fmt.Errorf("failed retrieving valid system status api response from sonarr: %s",
			resp.Response().Status)
	}

	// decode response
	var s SonarrSystemStatus
	if err := resp.ToJSON(&s); err != nil {
		return nil, errors.WithMessage(err, "failed decoding system status api response from sonarr")
	}

	return &s, nil
}

/* Interface Implements */

func (p *Sonarr) Init() error {
	// retrieve system status
	status, err := p.getSystemStatus()
	if err != nil {
		return errors.Wrap(err, "failed initializing sonarr pvr")
	}

	// determine version
	switch status.Version[0:1] {
	case "2":
		p.version = 2
	case "3":
		p.version = 3
	default:
		return errors.New("failed to determine version of sonarr pvr")
	}
	return nil
}

func (p *Sonarr) GetQueueSize() (int, error) {
	// send request
	resp, err := web.GetResponse(web.GET, web.JoinURL(p.apiUrl, "/queue"), 15, p.reqHeaders)
	if err != nil {
		return 0, errors.WithMessage(err, "failed retrieving queue api response from sonarr")
	}
	defer resp.Response().Body.Close()

	// validate response
	if resp.Response().StatusCode != 200 {
		return 0, fmt.Errorf("failed retrieving valid queue api response from sonarr: %s",
			resp.Response().Status)
	}

	// decode response
	var q SonarrQueue
	if err := resp.ToJSON(&q); err != nil {
		return 0, errors.WithMessage(err, "failed decoding queue api response from sonarr")
	}

	return q.Size, nil
}

func (p *Sonarr) GetWantedMissing() (map[int]MediaItem, error) {
	// logic vars
	totalRecords := 0
	wantedMissing := make(map[int]MediaItem, 0)

	page := 1
	lastPageSize := pvrDefaultPageSize

	// retrieve all page results
	p.log.Info("Retrieving wanted missing media...")

	for {
		// break loop when all pages retrieved
		if lastPageSize < pvrDefaultPageSize {
			break
		}

		// set params
		params := req.QueryParam{
			"sortKey":  pvrDefaultSortKey,
			"sortDir":  pvrDefaultSortDirection,
			"page":     page,
			"pageSize": pvrDefaultPageSize,
		}

		// send request
		resp, err := web.GetResponse(web.GET, web.JoinURL(p.apiUrl, "/wanted/missing"), 15,
			p.reqHeaders, params)
		if err != nil {
			return nil, errors.WithMessage(err, "failed retrieving wanted missing api response from sonarr")
		}

		// validate response
		if resp.Response().StatusCode != 200 {
			resp.Response().Body.Close()
			return nil, fmt.Errorf("failed retrieving valid wantedm issing api response from sonarr: %s",
				resp.Response().Status)
		}

		// decode response
		var m SonarrWanted
		if err := resp.ToJSON(&m); err != nil {
			resp.Response().Body.Close()
			return nil, errors.WithMessage(err, "failed decoding wanted missing api response from sonarr")
		}

		// process response
		lastPageSize = len(m.Records)
		for _, episode := range m.Records {
			// skip unmonitored episode
			if !episode.Monitored {
				continue
			}

			// store this episode
			airDate := episode.AirDateUtc
			wantedMissing[episode.Id] = MediaItem{
				AirDateUtc: &airDate,
				LastSearch: nil,
				Name: fmt.Sprintf("%s - S%02dE%02d", episode.Title, episode.SeasonNumber,
					episode.EpisodeNumber),
			}
		}
		totalRecords += lastPageSize

		p.log.WithField("page", page).Debug("Retrieved")
		page += 1

		// close response
		resp.Response().Body.Close()
	}

	p.log.WithField("media_items", totalRecords).Info("Finished")

	return wantedMissing, nil
}
