package nodemanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GeoData represents geolocation information from IPinfo.io
type GeoData struct {
	IP       string `json:"ip"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Loc      string `json:"loc"`
	Org      string `json:"org"`
	Timezone string `json:"timezone"`
}

// ParsedGeoData represents processed geo information
type ParsedGeoData struct {
	Country     string
	CountryName string
	City        string
	Region      string
	Latitude    float64
	Longitude   float64
	ASN         int
	ISP         string
}

// FetchGeoData fetches geolocation data from IPinfo.io
func (nm *NodeManager) fetchGeoData(ip string) (*ParsedGeoData, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	url := fmt.Sprintf("https://ipinfo.io/%s/json", ip)
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch geo data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geo API returned status: %d", resp.StatusCode)
	}

	var geoData GeoData
	if err := json.NewDecoder(resp.Body).Decode(&geoData); err != nil {
		return nil, fmt.Errorf("failed to decode geo data: %v", err)
	}

	// Parse the data
	parsed := &ParsedGeoData{
		Country:     geoData.Country,
		CountryName: geoData.Country, // Basic mapping - same as country code
		City:        geoData.City,
		Region:      geoData.Region,
	}

	// Parse coordinates from "lat,lng" format
	if geoData.Loc != "" {
		parts := strings.Split(geoData.Loc, ",")
		if len(parts) == 2 {
			if lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err == nil {
				parsed.Latitude = lat
			}
			if lng, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
				parsed.Longitude = lng
			}
		}
	}

	// Parse ASN and ISP from org field (format: "AS15169 Google LLC")
	if geoData.Org != "" {
		parts := strings.SplitN(geoData.Org, " ", 2)
		if len(parts) >= 1 && strings.HasPrefix(parts[0], "AS") {
			if asn, err := strconv.Atoi(parts[0][2:]); err == nil {
				parsed.ASN = asn
			}
		}
		if len(parts) >= 2 {
			parsed.ISP = parts[1]
		}
	}

	return parsed, nil
}

// UpdateNodeGeoData updates a node's geographic information
func (nm *NodeManager) updateNodeGeoData(nodeID string, geoData *ParsedGeoData) error {
	query := `
		UPDATE nodes 
		SET country = $2, country_name = $3, city = $4, region = $5,
		    latitude = $6, longitude = $7, asn = $8, isp = $9, updated_at = NOW()
		WHERE id = $1
	`
	
	_, err := nm.db.Exec(query, nodeID, 
		geoData.Country, geoData.CountryName, geoData.City, geoData.Region,
		geoData.Latitude, geoData.Longitude, geoData.ASN, geoData.ISP)
	
	if err != nil {
		return fmt.Errorf("failed to update node geo data: %v", err)
	}

	nm.logger.Infof("Updated geo data for node %s: %s, %s", nodeID, geoData.City, geoData.Country)
	return nil
}

// CheckAndFillGeoData checks if a node needs geo data and fetches it if missing
func (nm *NodeManager) checkAndFillGeoData(node *Node) {
	// Skip if geo data is already present
	if node.Country != "" && node.City != "" {
		return
	}

	// Skip IPv6 link-local and private addresses that won't have useful geo data
	if strings.HasPrefix(node.IPAddress, "fe80:") || 
	   strings.HasPrefix(node.IPAddress, "::1") ||
	   strings.HasPrefix(node.IPAddress, "127.") ||
	   strings.HasPrefix(node.IPAddress, "10.") ||
	   strings.HasPrefix(node.IPAddress, "192.168.") ||
	   (strings.HasPrefix(node.IPAddress, "172.") && len(node.IPAddress) > 4) {
		return
	}

	nm.logger.Debugf("Fetching geo data for node %s (IP: %s)", node.ID, node.IPAddress)

	geoData, err := nm.fetchGeoData(node.IPAddress)
	if err != nil {
		nm.logger.Warnf("Failed to fetch geo data for node %s: %v", node.ID, err)
		return
	}

	if geoData.Country == "" {
		nm.logger.Warnf("No geo data available for IP %s", node.IPAddress)
		return
	}

	// Update the database
	if err := nm.updateNodeGeoData(node.ID, geoData); err != nil {
		nm.logger.Errorf("Failed to update geo data for node %s: %v", node.ID, err)
		return
	}

	// Update the node struct for Redis caching
	node.Country = geoData.Country
	node.CountryName = geoData.CountryName
	node.City = geoData.City
	node.Region = geoData.Region
	node.Latitude = geoData.Latitude
	node.Longitude = geoData.Longitude
	node.ASN = geoData.ASN
	node.ISP = geoData.ISP
}