package hyperfeed

import (
	"net/url"
	"os"
	"testing"
)

// use the netnewswire default feeds file for test data
// source: https://github.com/Ranchero-Software/NetNewsWire/blob/main/Shared/Importers/DefaultFeeds.opml
var NNWDefaultOPML_sampledata string = `
<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
<head>
	<title>Default Feeds</title>
</head>
<body>
	<outline text="Allen Pike" title="Allen Pike" type="rss" version="RSS" htmlUrl="https://www.allenpike.com/" xmlUrl="https://feeds.allenpike.com/feed/"/>
	<outline text="Colossal" title="Colossal" type="rss" version="RSS" htmlUrl="https://www.thisiscolossal.com/" xmlUrl="https://www.thisiscolossal.com/feed/"/>
	<outline text="Craig Hockenberry" title="Craig Hockenberry" type="rss" version="RSS" htmlUrl="https://furbo.org/" xmlUrl="https://furbo.org/feed/json"/>
	<outline text="Daring Fireball" title="Daring Fireball" type="rss" version="RSS" htmlUrl="https://daringfireball.net/" xmlUrl="https://daringfireball.net/feeds/json"/>
	<outline text="inessential" title="inessential" type="rss" version="RSS" htmlUrl="https://inessential.com/" xmlUrl="https://inessential.com/feed.json"/>
	<outline text="Jason Kottke" title="Jason Kottke" type="rss" version="RSS" htmlUrl="https://kottke.org/" xmlUrl="https://feeds.kottke.org/json"/>
	<outline text="Julia Evans" title="Julia Evans" type="rss" version="RSS" htmlUrl="https://jvns.ca/" xmlUrl="https://jvns.ca/atom.xml"/>
	<outline text="Manton Reece" title="Manton Reece" type="rss" version="RSS" htmlUrl="https://manton.org/" xmlUrl="https://www.manton.org/feed/json"/>
	<outline text="Maurice Parker" title="Maurice Parker" type="rss" version="RSS" htmlUrl="https://vincode.io/" xmlUrl="https://vincode.io/feed.xml"/>
	<outline text="Michael Tsai" title="Michael Tsai" type="rss" version="RSS" htmlUrl="https://mjtsai.com/blog/" xmlUrl="https://mjtsai.com/blog/feed/"/>
	<outline text="NetNewsWire Blog" title="NetNewsWire Blog" type="rss" version="RSS" htmlUrl="https://netnewswire.blog/" xmlUrl="https://netnewswire.blog/feed.json"/>
	<outline text="One Foot Tsunami" title="One Foot Tsunami" type="rss" version="RSS" htmlUrl="https://onefoottsunami.com/" xmlUrl="https://onefoottsunami.com/feed/json/"/>
	<outline text="Scripting News" title="Scripting News" type="rss" version="RSS" htmlUrl="http://scripting.com/" xmlUrl="http://scripting.com/rss.xml"/>
	<outline text="Six Colors" title="Six Colors" type="rss" version="RSS" htmlUrl="https://sixcolors.com/" xmlUrl="https://feedpress.me/sixcolors?type=xml"/>
</body>
</opml>
`

func TestSubscriptionListLoad(t *testing.T) {
	var subscriptionList SubscriptionList
	var sampleDataFile *os.File
	var sampleDataFileURL *url.URL
	var err error
	if sampleDataFile, err = os.CreateTemp("/tmp/", "default.opml"); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(sampleDataFile.Name())
	if _, err := sampleDataFile.Write([]byte(NNWDefaultOPML_sampledata)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if sampleDataFileURL, err = url.Parse(sampleDataFile.Name()); err != nil {
		t.Fatalf("Failed to parse sampleDataFile URL: %v", err)
	}
	subscriptionList = SubscriptionList{URI: sampleDataFileURL}
	if err = subscriptionList.Load(); err != nil {
		t.Fatalf("Failed to load subscription list: %v", err)
	}
	if len(subscriptionList.OPML.Body.Outlines) != 14 {
		t.Fatalf("Expected 14 outlines; got %d", len(subscriptionList.OPML.Body.Outlines))
	}
}
