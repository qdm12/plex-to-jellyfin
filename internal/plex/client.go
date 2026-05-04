package plex

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type Settings struct {
	Client  *http.Client
	BaseURL string
	Token   string
}

type Client struct {
	client  *http.Client
	baseURL *url.URL
	token   string
}

func New(settings Settings) *Client {
	baseURL, _ := url.Parse(strings.TrimRight(settings.BaseURL, "/"))
	return &Client{
		client:  settings.Client,
		baseURL: baseURL,
		token:   settings.Token,
	}
}

func (c *Client) FetchSections(ctx context.Context) (sections []Section, err error) {
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, "/library/sections")
	q := u.Query()
	q.Set("X-Plex-Token", c.token)
	u.RawQuery = q.Encode()

	var body struct {
		Directories []struct {
			Key   string `xml:"key,attr"`
			Type  string `xml:"type,attr"`
			Title string `xml:"title,attr"`
		} `xml:"Directory"`
	}
	err = c.getXML(ctx, u.String(), &body)
	if err != nil {
		return nil, err
	}

	for _, directory := range body.Directories {
		sections = append(sections, Section{
			Key:   directory.Key,
			Type:  directory.Type,
			Title: directory.Title,
		})
	}

	return sections, nil
}

type xmlVideo struct {
	Type       string `xml:"type,attr"`
	Title      string `xml:"title,attr"`
	ViewCount  uint   `xml:"viewCount,attr"`
	ViewOffset uint64 `xml:"viewOffset,attr"`
	GUID       string `xml:"guid,attr"`
	Guids      []struct {
		ID string `xml:"id,attr"`
	} `xml:"Guid"`
	Media []struct {
		Part struct {
			File string `xml:"file,attr"`
		} `xml:"Part"`
	} `xml:"Media"`
}

func (c *Client) FetchSectionItems(ctx context.Context, sectionKey, sectionType string,
) (items []Item, err error) {
	u := *c.baseURL
	endpoint := "all"
	if sectionType == "show" {
		endpoint = "allLeaves"
	}
	u.Path = path.Join(c.baseURL.Path, "/library/sections", sectionKey, endpoint)
	query := u.Query()
	query.Set("X-Plex-Token", c.token)
	query.Set("includeGuids", "1")
	u.RawQuery = query.Encode()

	var body struct {
		Video []xmlVideo `xml:"Video"`
	}
	err = c.getXML(ctx, u.String(), &body)
	if err != nil {
		return nil, err
	}

	items = make([]Item, 0, len(body.Video))
	for _, metadata := range body.Video {
		item, ok := parseMetadata(metadata)
		if !ok {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

func (c *Client) getXML(ctx context.Context, address string, parsed any) (err error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("doing request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(response.Body)
		return fmt.Errorf("unexpected status code %d: %s", response.StatusCode, strings.TrimSpace(string(payload)))
	}

	decoder := xml.NewDecoder(response.Body)
	err = decoder.Decode(parsed)
	if err != nil {
		return fmt.Errorf("decoding XML response: %w", err)
	}

	return nil
}
