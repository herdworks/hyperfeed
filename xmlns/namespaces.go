package xmlns

// VARIABLES
// =============================================================================

// Extension namespaces
const (
	// iTunes Podcasts: https://podcasters.apple.com/support/823-podcast-requirements
	ItunesPodcastNS string = "http://www.itunes.com/dtds/podcast-1.0.dtd"

	// Media RSS
	// https://www.rssboard.org/media-rss
	YahooMediaRSSNS string = "http://search.yahoo.com/mrss/"

	// Dublin Core
	DublinCoreTermsNS    string = "http://purl.org/dc/terms/"
	DublicCoreMetadataNS string = "http://purl.org/dc/elements/1.1/"

	// Scripting
	ScriptingNewsSourceNS string = "http://source.scripting.com/"

	// Google Play Podcasts
	// https://www.google.com/schemas/play-podcasts/1.0/play-podcasts.xsd
	// https://support.google.com/podcast-publishers/answer/9889544?visit_id=638961521969445342-2535738675&rd=1
	GooglePlayPodcastsNS string = "http://www.google.com/schemas/play-podcasts/1.0"

	// YouTube XML
	YouTubeNS string = "http://www.youtube.com/xml/schemas/2015"

	// PodcastIndex ("Podcasting 2.0")
	PodcastIndexNS string = "https://podcastindex.org/namespace/1.0"

	// Other common extension namespaces
	AtomNS       string = "http://www.w3.org/2005/Atom"
	RSSNS        string = "http://purl.org/rss/1.0/"
	RSSContentNS string = "http://purl.org/rss/1.0/modules/content/"
)

// METHODS
// =============================================================================

func (extensions Extensions) GetItunesMetadata(name string) *Extension {
	return extensions.GetProperty(ItunesPodcastNS, name)
}

func (extensions Extensions) GetPodcastIndexMetadata(name string) *Extension {
	return extensions.GetProperty(PodcastIndexNS, name)
}

func (extensions Extensions) GetGooglePlayMetadata(name string) *Extension {
	return extensions.GetProperty(GooglePlayPodcastsNS, name)
}

func (extensions Extensions) GetYahooMediaRSSMetadata(name string) *Extension {
	return extensions.GetProperty(YahooMediaRSSNS, name)
}

func (extensions Extensions) GetRSSContentMetadata(name string) *Extension {
	return extensions.GetProperty(RSSContentNS, name)
}

func (extensions Extensions) GetDublinCoreMetadata(name string) *Extension {
	return extensions.GetProperty(DublicCoreMetadataNS, name)
}

// Prefix returns the conventional XML prefix for a supported extension namespace.
func Prefix(namespace string) string {
	switch namespace {
	case AtomNS:
		return "atom"
	case RSSContentNS:
		return "content"
	case DublicCoreMetadataNS:
		return "dc"
	case ItunesPodcastNS:
		return "itunes"
	case YahooMediaRSSNS:
		return "media"
	case PodcastIndexNS:
		return "podcast"
	default:
		return ""
	}
}
