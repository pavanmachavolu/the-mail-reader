package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"
)

type Message struct {
	ID            string     `json:"id"`
	Subject       string     `json:"subject"`
	From          string     `json:"from"`
	To            []string   `json:"to"`
	ReceivedAt    time.Time  `json:"received_at"`
	BodyText      string     `json:"body_text,omitempty"`
	BodyMarkdown  string     `json:"body_markdown,omitempty"`
	HasAttachment bool       `json:"has_attachment"`
	Attachments   []Attachment `json:"attachments,omitempty"`
	Folder        string     `json:"folder"`
	Read          bool       `json:"read"`
	Importance    string     `json:"importance,omitempty"`
	Categories    []string   `json:"categories,omitempty"`
}

type Attachment struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
}

type graphMessage struct {
	ID         string `json:"id"`
	Subject    string `json:"subject"`
	From       struct {
		EmailAddress struct {
			Address string `json:"address"`
			Name    string `json:"name"`
		} `json:"emailAddress"`
	} `json:"from"`
	ToRecipients []struct {
		EmailAddress struct {
			Address string `json:"address"`
			Name    string `json:"name"`
		} `json:"emailAddress"`
	} `json:"toRecipients"`
	ReceivedDateTime   string `json:"receivedDateTime"`
	Body              struct {
		Content     string `json:"content"`
		ContentType string `json:"contentType"`
	} `json:"body"`
	HasAttachments bool `json:"hasAttachments"`
	Attachments    []struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Size          int64  `json:"size"`
		ContentType   string `json:"contentType"`
	} `json:"attachments"`
	ParentFolderID string   `json:"parentFolderId"`
	IsRead         bool     `json:"isRead"`
	Importance     string   `json:"importance"`
	Categories     []string `json:"categories"`
}

type graphMessageList struct {
	Value    []graphMessage `json:"value"`
	NextLink string         `json:"@odata.nextLink"`
}

type folder struct {
	ID         string `json:"id"`
	DisplayName string `json:"displayName"`
}

type folderList struct {
	Value []folder `json:"value"`
}

func (c *Config) getAccessToken() (string, error) {
	if c.AccessToken != "" && c.ExpiresAt.After(time.Now().Add(2*time.Minute)) {
		return c.AccessToken, nil
	}
	err := RefreshAccessToken(c)
	if err != nil {
		return "", err
	}
	return c.AccessToken, nil
}

func graphRequest(config *Config, method, path string, query url.Values) ([]byte, error) {
	token, err := config.getAccessToken()
	if err != nil {
		return nil, err
	}

	urlStr := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		urlStr = microsoftGraphURL + path
	}
	if query != nil {
		urlStr += "?" + query.Encode()
	}

	timeout := 300 * time.Second
	if v := os.Getenv("MAILREADER_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}

		req, err := http.NewRequest(method, urlStr, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{Timeout: timeout}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("graph API error %d: %s", resp.StatusCode, string(body))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("graph API error %d: %s", resp.StatusCode, string(body))
		}

		return body, nil
	}

	return nil, lastErr
}

func getFolders(config *Config) ([]folder, error) {
	query := url.Values{
		"$select": {"id,displayName"},
	}
	body, err := graphRequest(config, "GET", "/me/mailFolders", query)
	if err != nil {
		return nil, err
	}

	var folders folderList
	if err := json.Unmarshal(body, &folders); err != nil {
		return nil, err
	}
	return folders.Value, nil
}

func fetchMessages(config *Config, folderID string, folderName string, top int, filter string) ([]Message, error) {
	var messages []Message
	firstPath := fmt.Sprintf("/me/mailFolders/%s/messages", url.PathEscape(folderID))
	firstQuery := url.Values{
		"$select": {"id,subject,from,toRecipients,receivedDateTime,body,hasAttachments,attachments,parentFolderId,isRead,importance,categories,conversationId"},
		"$top":    {fmt.Sprintf("%d", top)},
	}
	if filter != "" {
		firstQuery.Set("$filter", filter)
	}

	path := firstPath
	query := firstQuery

	for {
		body, err := graphRequest(config, "GET", path, query)
		if err != nil {
			return nil, err
		}

		var list graphMessageList
		if err := json.Unmarshal(body, &list); err != nil {
			return nil, err
		}

		for i := range list.Value {
			msg := list.Value[i]

			markdown, _ := md.ConvertString(msg.Body.Content)

			converted := Message{
				ID:             msg.ID,
				Subject:        msg.Subject,
				From:           msg.From.EmailAddress.Address,
				ReceivedAt:     time.Time{},
				BodyMarkdown:   markdown,
				HasAttachment:  msg.HasAttachments,
				Folder:         folderName,
				Read:           msg.IsRead,
				Importance:     msg.Importance,
				Categories:     msg.Categories,
			}

			if msg.Body.ContentType == "text" {
				converted.BodyText = msg.Body.Content
			}

			for _, r := range msg.ToRecipients {
				converted.To = append(converted.To, r.EmailAddress.Address)
			}

			if t, err := time.Parse(time.RFC3339, msg.ReceivedDateTime); err == nil {
				converted.ReceivedAt = t
			}

			for _, a := range msg.Attachments {
				converted.Attachments = append(converted.Attachments, Attachment{
					ID:          a.ID,
					Name:        a.Name,
					SizeBytes:   a.Size,
					ContentType: a.ContentType,
				})
			}

			messages = append(messages, converted)
		}

		if list.NextLink == "" {
			break
		}
		path = list.NextLink
		query = nil
	}

	return messages, nil
}
