package generator

import (
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/herdworks/hyperfeed"
	"github.com/herdworks/hyperfeed/rss"
	"github.com/herdworks/hyperfeed/xmlns"
)

// RSSGenerator serializes normalized HyperFeed values as RSS 2.0 XML.
type RSSGenerator struct{}

type rssExtensionUse struct {
	atom    bool
	content bool
	dublin  bool
	itunes  bool
	media   bool
}

// Generate serializes feed as an RSS 2.0 document.
func (RSSGenerator) Generate(feed *hyperfeed.Feed) ([]byte, error) {
	if feed == nil {
		return nil, errors.New("rss generator requires a feed")
	}
	if feed.URL == nil || feed.URL.String() == "" {
		return nil, errors.New("rss generator requires feed URL")
	}
	atomLinks, err := rssAtomLinks(feed.Links)
	if err != nil {
		return nil, err
	}
	channel := rss.Channel{
		Title:          feed.Title,
		Links:          append([]rss.Link{{Href: rssFeedLink(feed)}}, atomLinks...),
		Description:    feed.Description,
		Language:       feed.Language,
		LastBuildDate:  rssTimestamp(feed.ModifiedAt),
		ManagingEditor: rssManagingEditor(feed.Bylines),
		Categories:     rssCategories(feed.Categories),
	}
	if cover := rssCoverURL(feed, nil); cover != "" {
		channel.Image = &rss.Image{URL: cover, Title: feed.Title, Link: rssFeedLink(feed)}
	}
	result := &rss.RSS{XMLName: xml.Name{Local: "rss"}, Version: "2.0", Channel: channel}
	if len(atomLinks) > 0 {
		result.AtomNS = xmlns.AtomNS
	}
	if rssPodcastEnabled(feed) {
		result.ItunesNS = xmlns.ItunesPodcastNS
		channel.Extensions = append(channel.Extensions,
			rssExtension(xmlns.ItunesPodcastNS, "summary", feed.Description),
			rssExtension(xmlns.ItunesPodcastNS, "author", strings.Join(rssBylineNames(feed.Bylines), ", ")),
			rssExtension(xmlns.ItunesPodcastNS, "explicit", strconv.FormatBool(rssFeedExplicit(feed))),
			rssExtension(xmlns.ItunesPodcastNS, "subtitle", feed.Description),
		)
		if cover := rssCoverURL(feed, nil); cover != "" {
			channel.Extensions = append(channel.Extensions, rssExtension(xmlns.ItunesPodcastNS, "image", "", rssAttribute("href", cover)))
		}
		if owner := rssItunesOwner(feed.Bylines); owner.XMLName.Local != "" {
			channel.Extensions = append(channel.Extensions, owner)
		}
		for _, category := range rssCategoryLabels(feed.Categories) {
			channel.Extensions = append(channel.Extensions, rssExtension(xmlns.ItunesPodcastNS, "category", "", rssAttribute("text", category)))
		}
		if feed.Medium != "" {
			result.PodcastNS = xmlns.PodcastIndexNS
			channel.Extensions = append(channel.Extensions, rssExtension(xmlns.PodcastIndexNS, "medium", feed.Medium))
		}
	}
	for _, item := range feed.Items {
		generated, uses, err := rssItem(feed, item)
		if err != nil {
			return nil, err
		}
		if uses.content {
			result.ContentNS = xmlns.RSSContentNS
		}
		if uses.dublin {
			result.DublinNS = xmlns.DublicCoreMetadataNS
		}
		if uses.atom {
			result.AtomNS = xmlns.AtomNS
		}
		if uses.itunes {
			result.ItunesNS = xmlns.ItunesPodcastNS
		}
		if uses.media {
			result.MediaNS = xmlns.YahooMediaRSSNS
		}
		channel.Items = append(channel.Items, generated)
	}
	result.Channel = channel
	return xml.MarshalIndent(result, "", "\t")
}

// RSS serializes a normalized feed as an RSS 2.0 document.
func RSS(feed *hyperfeed.Feed) ([]byte, error) {
	return RSSGenerator{}.Generate(feed)
}

func rssItem(feed *hyperfeed.Feed, item *hyperfeed.Item) (rss.Item, rssExtensionUse, error) {
	var result rss.Item
	var uses rssExtensionUse
	if item == nil {
		return result, uses, errors.New("rss generator cannot serialize a nil item")
	}
	result.Title = item.Title
	if link := rssItemLink(item); link != "" {
		result.Links = append(result.Links, rss.Link{Href: link})
	}
	atomLinks, err := rssAtomLinks(item.Links)
	if err != nil {
		return rss.Item{}, uses, err
	}
	result.Links = append(result.Links, atomLinks...)
	uses.atom = len(atomLinks) > 0
	result.Description = item.Summary
	if item.Content != "" {
		result.Extensions = append(result.Extensions, rssExtension(xmlns.RSSContentNS, "encoded", item.Content))
		uses.content = true
	}
	for _, byline := range rssBylineNames(item.Bylines) {
		result.Extensions = append(result.Extensions, rssExtension(xmlns.DublicCoreMetadataNS, "creator", byline))
		uses.dublin = true
	}
	if item.CommentsURL != nil {
		result.Comments = rss.Text{Value: item.CommentsURL.String()}
	}
	if item.GUID != "" {
		result.GUID = rss.RSSGUID{Value: item.GUID, IsPermaLink: true}
		if len(result.Links) > 0 && item.GUID != result.Links[0].Href {
			result.GUID.IsPermaLink = false
		}
	}
	result.PubDate = rssTimestamp(item.PublishedAt)
	result.Categories = rssCategories(item.Categories)
	if result.Enclosures, err = rssEnclosures(item.Attachments); err != nil {
		return rss.Item{}, uses, err
	}
	if rssPodcastEnabled(feed) {
		itemExtensions := xmlns.Extensions{
			rssExtension(xmlns.ItunesPodcastNS, "author", strings.Join(rssBylineNames(item.Bylines), ", ")),
			rssExtension(xmlns.ItunesPodcastNS, "summary", item.Summary),
		}
		if item.Explicit != nil {
			itemExtensions = append(itemExtensions, rssExtension(xmlns.ItunesPodcastNS, "explicit", strconv.FormatBool(*item.Explicit)))
		}
		if cover := rssCoverURL(feed, item); cover != "" {
			itemExtensions = append(itemExtensions, rssExtension(xmlns.ItunesPodcastNS, "image", "", rssAttribute("href", cover)))
		}
		if duration := rssEpisodeDuration(item.Attachments); duration != "" {
			itemExtensions = append(itemExtensions, rssExtension(xmlns.ItunesPodcastNS, "duration", duration))
		}
		for _, extension := range itemExtensions {
			if extension.Text != "" || len(extension.Attrs) > 0 {
				result.Extensions = append(result.Extensions, extension)
				uses.itunes = true
			}
		}
	}
	media := rssMedia(item.Attachments)
	if cover := rssCoverURL(feed, item); cover != "" {
		media = append(media, rssExtension(xmlns.YahooMediaRSSNS, "thumbnail", "", rssAttribute("url", cover)))
	}
	if keywords := strings.Join(rssCategoryLabels(item.Categories), ","); keywords != "" {
		media = append(media, rssExtension(xmlns.YahooMediaRSSNS, "keywords", keywords))
	}
	if len(media) > 0 {
		result.Extensions = append(result.Extensions, media...)
		uses.media = true
	}
	return result, uses, nil
}

func rssFeedLink(feed *hyperfeed.Feed) string {
	if feed.HTMLURL != nil && feed.HTMLURL.String() != "" {
		return feed.HTMLURL.String()
	}
	for _, link := range feed.Links {
		if link != nil && link.Href != nil && strings.EqualFold(link.Rel, "alternate") {
			return link.Href.String()
		}
	}
	return feed.URL.String()
}

func rssItemLink(item *hyperfeed.Item) string {
	if item.URL != nil && item.URL.String() != "" {
		return item.URL.String()
	}
	for _, link := range item.Links {
		if link != nil && link.Href != nil && strings.EqualFold(link.Rel, "alternate") {
			return link.Href.String()
		}
	}
	return ""
}

func rssFeedExplicit(feed *hyperfeed.Feed) bool {
	for _, item := range feed.Items {
		if item != nil && item.Explicit != nil && *item.Explicit {
			return true
		}
	}
	return false
}

func rssPodcastEnabled(feed *hyperfeed.Feed) bool {
	if feed.IsPodcast || feed.Medium != "" {
		return true
	}
	for _, item := range feed.Items {
		if item != nil && item.Explicit != nil {
			return true
		}
	}
	return false
}

func rssCoverURL(feed *hyperfeed.Feed, item *hyperfeed.Item) string {
	if item != nil && item.CoverURL != nil && item.CoverURL.String() != "" {
		return item.CoverURL.String()
	}
	if feed.CoverURL != nil && feed.CoverURL.String() != "" {
		return feed.CoverURL.String()
	}
	if feed.Website.CoverURL != "" {
		return feed.Website.CoverURL
	}
	return feed.Website.FaviconURL
}

func rssAtomLinks(links []*hyperfeed.Link) ([]rss.Link, error) {
	result := make([]rss.Link, 0, len(links))
	for _, link := range links {
		if link == nil || link.Href == nil || link.Href.String() == "" {
			continue
		}
		if strings.EqualFold(link.Rel, "alternate") || strings.EqualFold(link.Rel, "comments") || strings.EqualFold(link.Rel, "replies") {
			continue
		}
		generated, err := rssAtomLink(*link)
		if err != nil {
			return nil, err
		}
		result = append(result, generated)
	}
	return result, nil
}

func rssAtomLink(link hyperfeed.Link) (rss.Link, error) {
	result := rss.Link{Href: link.Href.String(), Rel: link.Rel, Type: link.Mimetype, HrefLang: link.HrefLang, Title: link.Title, Space: xmlns.AtomNS}
	if link.Length == nil {
		return result, nil
	}
	if *link.Length < 0 {
		return rss.Link{}, fmt.Errorf("rss atom link %q has negative length", link.Href.String())
	}
	if uint64(*link.Length) > uint64(math.MaxUint) {
		return rss.Link{}, fmt.Errorf("rss atom link %q length exceeds platform uint", link.Href.String())
	}
	length := uint(*link.Length)
	result.Length = &length
	return result, nil
}

func rssEnclosures(attachments []hyperfeed.Attachment) ([]rss.Enclosure, error) {
	result := []rss.Enclosure{}
	for _, attachment := range attachments {
		if !attachment.Enclosure || attachment.Href == nil || attachment.Href.String() == "" {
			continue
		}
		if attachment.Size < 0 {
			return nil, fmt.Errorf("rss enclosure %q has negative size", attachment.Href.String())
		}
		result = append(result, rss.Enclosure{URL: attachment.Href.String(), Length: strconv.FormatInt(attachment.Size, 10), Type: attachment.Mimetype})
	}
	return result, nil
}

func rssBylineNames(bylines []hyperfeed.Byline) []string {
	result := []string{}
	for _, byline := range orderedBylines(bylines) {
		if byline.Name != "" {
			result = append(result, byline.Name)
		}
	}
	return result
}

func rssManagingEditor(bylines []hyperfeed.Byline) string {
	for _, byline := range orderedBylines(bylines) {
		if byline.Email != "" {
			return byline.Email
		}
	}
	return ""
}

func rssCategories(categories []hyperfeed.Category) []rss.Category {
	result := make([]rss.Category, 0, len(categories))
	for _, label := range rssCategoryLabels(categories) {
		result = append(result, rss.Category{Text: label})
	}
	return result
}

func rssCategoryLabels(categories []hyperfeed.Category) []string {
	result := []string{}
	for _, category := range categories {
		if category.Label != "" {
			result = append(result, category.Label)
		}
	}
	return result
}

func rssItunesOwner(bylines []hyperfeed.Byline) xmlns.Extension {
	for _, byline := range orderedBylines(bylines) {
		if byline.Email != "" {
			owner := rssExtension(xmlns.ItunesPodcastNS, "owner", "")
			owner.Extensions = append(owner.Extensions,
				rssExtension(xmlns.ItunesPodcastNS, "name", byline.Name),
				rssExtension(xmlns.ItunesPodcastNS, "email", byline.Email),
			)
			return owner
		}
	}
	return xmlns.Extension{}
}

func rssEpisodeDuration(attachments []hyperfeed.Attachment) string {
	for _, attachment := range attachments {
		if !attachment.Enclosure || !rssPodcastAttachment(attachment) {
			continue
		}
		if attachment.Duration > 0 {
			return strconv.FormatInt(attachment.Duration, 10)
		}
		return ""
	}
	return ""
}

func rssMedia(attachments []hyperfeed.Attachment) (result xmlns.Extensions) {
	for _, attachment := range attachments {
		if attachment.Href == nil || attachment.Href.String() == "" || !rssMediaAttachment(attachment) {
			continue
		}
		attributes := []xml.Attr{rssAttribute("url", attachment.Href.String())}
		if attachment.Mimetype != "" {
			attributes = append(attributes, rssAttribute("type", attachment.Mimetype))
		}
		if attachment.Size > 0 {
			attributes = append(attributes, rssAttribute("fileSize", strconv.FormatInt(attachment.Size, 10)))
		}
		if attachment.Duration > 0 {
			attributes = append(attributes, rssAttribute("duration", strconv.FormatInt(attachment.Duration, 10)))
		}
		if attachment.Width > 0 {
			attributes = append(attributes, rssAttribute("width", strconv.Itoa(attachment.Width)))
		}
		if attachment.Height > 0 {
			attributes = append(attributes, rssAttribute("height", strconv.Itoa(attachment.Height)))
		}
		result = append(result, rssExtension(xmlns.YahooMediaRSSNS, "content", "", attributes...))
		if attachment.Poster != nil && attachment.Poster.String() != "" {
			thumbnail := []xml.Attr{rssAttribute("url", attachment.Poster.String())}
			if attachment.Width > 0 {
				thumbnail = append(thumbnail, rssAttribute("width", strconv.Itoa(attachment.Width)))
			}
			if attachment.Height > 0 {
				thumbnail = append(thumbnail, rssAttribute("height", strconv.Itoa(attachment.Height)))
			}
			result = append(result, rssExtension(xmlns.YahooMediaRSSNS, "thumbnail", "", thumbnail...))
		}
	}
	return result
}

func rssExtension(namespace string, name string, text string, attributes ...xml.Attr) xmlns.Extension {
	return xmlns.Extension{XMLName: xml.Name{Space: namespace, Local: name}, Text: text, Attrs: attributes}
}

func rssAttribute(name string, value string) xml.Attr {
	return xml.Attr{Name: xml.Name{Local: name}, Value: value}
}

func rssMediaAttachment(attachment hyperfeed.Attachment) bool {
	return strings.HasPrefix(attachment.Mimetype, "image/") || strings.HasPrefix(attachment.Mimetype, "audio/") || strings.HasPrefix(attachment.Mimetype, "video/")
}

func rssPodcastAttachment(attachment hyperfeed.Attachment) bool {
	return strings.HasPrefix(attachment.Mimetype, "audio/") || strings.HasPrefix(attachment.Mimetype, "video/")
}

func rssTimestamp(timestamp time.Time) string {
	if timestamp.IsZero() {
		return ""
	}
	return timestamp.Format(time.RFC1123Z)
}
