package radiko

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/yyoshiki41/go-radiko/internal/m3u8"
	"github.com/yyoshiki41/go-radiko/internal/util"
)

type streamURL struct {
	AreaFree          int    `xml:"areafree,attr"`
	TimeFree          int    `xml:"timefree,attr"`
	PlaylistCreateUrl string `xml:"playlist_create_url"`
}

type streamInfo struct {
	Urls []streamURL `xml:"url"`
}

// TimeshiftPlaylistM3U8 returns uri.
func (c *Client) TimeshiftPlaylistM3U8(ctx context.Context, stationID string, start time.Time, ft, to string) (string, error) {
	// Fetch stream XML info
	xmlURL := fmt.Sprintf("https://radiko.jp/v3/station/stream/pc_html5/%s.xml", stationID)
	reqXML, err := http.NewRequest("GET", xmlURL, nil)
	if err != nil {
		return "", err
	}
	reqXML = reqXML.WithContext(ctx)
	reqXML.Header.Set("User-Agent", userAgent)

	respXML, err := c.httpClient.Do(reqXML)
	if err != nil {
		return "", err
	}
	defer respXML.Body.Close()

	var info streamInfo
	if err := xml.NewDecoder(respXML.Body).Decode(&info); err != nil {
		return "", err
	}

	var playlistURL string
	// Try to find areafree=0 (local) and timefree=1 first
	for _, u := range info.Urls {
		if u.TimeFree == 1 && u.AreaFree == 0 {
			playlistURL = u.PlaylistCreateUrl
			break
		}
	}
	// Fallback: any timefree=1
	if playlistURL == "" {
		for _, u := range info.Urls {
			if u.TimeFree == 1 {
				playlistURL = u.PlaylistCreateUrl
				break
			}
		}
	}

	if playlistURL == "" {
		return "", fmt.Errorf("no valid timeshift URL found for station %s", stationID)
	}

	// Create request to the fetched playlist URL
	req, err := http.NewRequest("GET", playlistURL, nil)
	if err != nil {
		return "", err
	}
	req = req.WithContext(ctx)

	// Generate random LSID
	lsidBytes := make([]byte, 16)
	_, _ = rand.Read(lsidBytes)
	lsid := hex.EncodeToString(lsidBytes)

	// Set query params
	q := req.URL.Query()
	q.Add("station_id", stationID)
	q.Add("l", "300")
	q.Add("ft", ft)
	q.Add("to", to)
	q.Add("start_at", ft) // Fixed to program start
	q.Add("end_at", to)
	q.Add("seek", util.Datetime(start)) // Dynamic seek position
	q.Add("lsid", lsid)
	q.Add("preroll", "0")
	q.Add("type", "b")
	req.URL.RawQuery = q.Encode()

	// Set headers
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("pragma", "no-cache")
	req.Header.Set(radikoAuthTokenHeader, c.AuthToken())

	resp, err := c.Do(req)
	if err != nil {
		return "", err	}
	defer resp.Body.Close()

	return m3u8.GetURI(resp.Body)
}

// GetTimeshiftURL returns a timeshift url for web browser.
func GetTimeshiftURL(stationID string, start time.Time) string {
	endpoint := path.Join("#!/ts", stationID, util.Datetime(start))
	return defaultEndpoint + "/" + endpoint
}
