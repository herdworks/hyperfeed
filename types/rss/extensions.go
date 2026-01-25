package rss

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// Extension preserves any XML element outside the core spec during parsing.
// It captures the qualified name, attributes, character data, and inner XML.
type Extension struct {
	XMLName    xml.Name
	Attrs      []xml.Attr  `json:"-"`
	Text       string      `json:"-"`
	InnerXML   string      `json:"-"`
	Extensions []Extension `xml:",any" json:"-"` // nested elements
}

// Extension namespaces
const (
	// iTunes Podcasts: https://podcasters.apple.com/support/823-podcast-requirements
	ExtensionNamespaceItunesPodcast string = "http://www.itunes.com/dtds/podcast-1.0.dtd"

	// PodcastIndex // "Podcasting 2.0"
	ExtensionNamespacePodcastIndex string = "https://podcastindex.org/namespace/1.0"

	// Google Play Podcasts
	// https://www.google.com/schemas/play-podcasts/1.0/play-podcasts.xsd
	// https://support.google.com/podcast-publishers/answer/9889544?visit_id=638961521969445342-2535738675&rd=1
	ExtensionNamespaceGooglePlayPodcasts string = "http://www.google.com/schemas/play-podcasts/1.0"

	// Media RSS
	// https://www.rssboard.org/media-rss
	ExtensionNamespaceYahooMediaRSS string = "http://search.yahoo.com/mrss/"

	// Dublin Core
	ExtensionNamespaceDublinCoreTerms    string = "http://purl.org/dc/terms/"
	ExtensionNamespaceDublicCoreMetadata string = "http://purl.org/dc/elements/1.1/"

	// Scripting
	ExtensionNamespaceScriptingNewsSource string = "http://source.scripting.com/"

	// Other common extension namespaces
	ExtensionNamespaceAtom       string = "http://www.w3.org/2005/Atom"
	ExtensionNamespaceRSSContent string = "http://purl.org/rss/1.0/modules/content/"

	// YouTube XML
	ExtensionNamespaceYouTube string = "http://www.youtube.com/xml/schemas/2015"
)

// ============================ Extension Methods =============================

// UnmarshalXML captures attributes, text, inner XML, and any nested extensions.
func (e *Extension) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var payload struct {
		Text  string      `xml:",chardata"`
		Inner string      `xml:",innerxml"`
		Any   []Extension `xml:",any"`
	}
	if err := d.DecodeElement(&payload, &start); err != nil {
		return err
	}
	e.XMLName = start.Name
	e.Attrs = append(e.Attrs[:0], start.Attr...)
	e.Text = payload.Text
	e.InnerXML = payload.Inner
	e.Extensions = append(e.Extensions[:0], payload.Any...)
	return nil
}

// func (r *RSS) ParseChannelExtensionAuthors() (authors []Author) {
// 	var author Author = Author{}
// 	if r.Channel.Title != "" {
// 		author.Name = r.Channel.Title // use feed title as default author name
// 	}
// 	if ownerMetadata := GetItunesMetadata(r.Channel.Extensions, "owner"); ownerMetadata != nil {
// 		if name := GetItunesMetadata(ownerMetadata.Extensions, "name"); name != nil {
// 			author.Name = name.Text
// 		}
// 		if email := GetItunesMetadata(ownerMetadata.Extensions, "email"); email != nil {
// 			author.Email = email.Text
// 		}
// 	}
// 	if !author.IsEmpty() {
// 		authors = append(authors, author)
// 	}
// 	return authors
// }
// func (r *RSS) ParseItemExtensionAuthors(rssitem *RSSItem) (authors []Author) {
// 	var author Author = Author{}
// 	if r.Channel.Title != "" {
// 		author.Name = r.Channel.Title
// 	}
// 	if ownerMetadata := GetItunesMetadata(r.Channel.Extensions, "owner"); ownerMetadata != nil {
// 		if name := GetItunesMetadata(ownerMetadata.Extensions, "name"); name != nil {
// 			author.Name = name.Text
// 		}
// 		if email := GetItunesMetadata(ownerMetadata.Extensions, "email"); email != nil {
// 			author.Email = email.Text
// 		}
// 	}
// 	if !author.IsEmpty() {
// 		authors = append(authors, author)
// 	}
// 	return authors
// }

func GetExtensionProperties(extensions []Extension, namespace string) (properties []*Extension) {
	for _, ext := range extensions {
		if ext.XMLName.Space == namespace {
			properties = append(properties, &ext)
		}
	}
	return properties
}

func GetExtensionProperty(extensions []Extension, namespace string, name string) *Extension {
	for _, ext := range extensions {
		if ext.XMLName.Space == namespace && ext.XMLName.Local == name {
			return &ext
		}
	}
	return nil
}

func GetItunesMetadata(extensions []Extension, name string) *Extension {
	return GetExtensionProperty(extensions, ExtensionNamespaceItunesPodcast, name)
}

func GetPodcastIndexMetadata(extensions []Extension, name string) *Extension {
	return GetExtensionProperty(extensions, ExtensionNamespacePodcastIndex, name)
}

func GetGooglePlayMetadata(extensions []Extension, name string) *Extension {
	return GetExtensionProperty(extensions, ExtensionNamespaceGooglePlayPodcasts, name)
}

func GetYahooMediaRSSMetadata(extensions []Extension, name string) *Extension {
	return GetExtensionProperty(extensions, ExtensionNamespaceYahooMediaRSS, name)
}

func GetRSSContentMetadata(extensions []Extension, name string) *Extension {
	return GetExtensionProperty(extensions, ExtensionNamespaceRSSContent, name)
}

func GetDublinCoreMetadata(extensions []Extension, name string) *Extension {
	return GetExtensionProperty(extensions, ExtensionNamespaceDublicCoreMetadata, name)
}

func GetItunesOwner(metadata []Extension) (author string) {
	var name, email string
	if ownerMetadata := GetItunesMetadata(metadata, "owner"); ownerMetadata != nil {
		if n := GetItunesMetadata(ownerMetadata.Extensions, "name"); n != nil {
			name = n.Text
		}
		if e := GetItunesMetadata(ownerMetadata.Extensions, "email"); e != nil {
			email = e.Text
		}
	}
	return fmt.Sprintf("%s <%s>", name, email)
}

func GetPodcastEpisodeDurationSeconds(item *Item) (seconds int) {
	var duration *Extension
	if duration = GetItunesMetadata(item.Extensions, "duration"); duration != nil {
		return GetItunesDurationSeconds(duration)
	} else if duration = GetPodcastIndexMetadata(item.Extensions, "duration"); duration != nil {
		return GetItunesDurationSeconds(duration)
	} else if duration = GetYahooMediaRSSMetadata(item.Extensions, "duration"); duration != nil {
		return GetItunesDurationSeconds(duration)
	} else if duration = GetGooglePlayMetadata(item.Extensions, "duration"); duration != nil {
		return GetItunesDurationSeconds(duration)
	}
	return 0
}
func GetItunesDurationSeconds(duration *Extension) (seconds int) {
	if duration == nil {
		return 0
	}
	switch d := GetItunesDurationDigits(duration.Text); len(d) {
	case 3:
		// HH:MM:SS format
		seconds += d[0] * 3600
		seconds += d[1] * 60
		seconds += d[2]
		return seconds
	case 2:
		// MM:SS format
		seconds += d[0] * 60
		seconds += d[1]
		return seconds
	case 1:
		// SS format
		return d[0]
	default:
		return 0
	}
}

func GetItunesDurationDigits(duration string) (parts []int) {
	for _, part := range strings.Split(duration, ":") {
		if p, err := strconv.Atoi(part); err != nil {
			return []int{0}
		} else {
			parts = append(parts, p)
		}
	}
	return parts
}

func GetDublinCoreCreator(metadata []Extension) (creator string) {
	if creatorMetadata := GetDublinCoreMetadata(metadata, "creator"); creatorMetadata != nil {
		creator = creatorMetadata.Text
	}
	return creator
}
