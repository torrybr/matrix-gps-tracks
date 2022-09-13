// Copyright (C) 2017 Tulir Asokan
// Copyright (C) 2018-2020 Luca Weiss
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"bufio"
	"flag"
	"fmt"
	geojson "github.com/paulmach/go.geojson"
	"io/ioutil"
	"log"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
	"os"
	"strings"
	"time"
)

var homeserver = flag.String("homeserver", "https://matrix.org", "matrix homeserver")

//var username = flag.String("username", "tbxx", "Matrix username localpart")
//
//var password = flag.String("password", "uwj4fqw7bzt.UDK_ntu", "Matrix password")

//var password = flag.String("password", "!0h#x33TX5vW", "Matrix password")

//var gpsTestRoom = "!TMVjAXEdloWyWhsGAJ:matrix.org"

var gpsTestRoom = "!YUsODcJXHpRWYkvRPA:matrix.org"

var geos = getGeoJsons()

type MyLocation struct {
	Body      string                `json:"body"`
	Msgtype   string                `json:"msgtype"`
	GeoURI    string                `json:"geo_uri"`
	MLocation MLocation             `json:"m.location"`
	MAsset    OrgMatrixMsc3488Asset `json:"m.asset"`
	MText     string                `json:"m.text"`
	MTs       int64                 `json:"m.ts"`
}

type MLocation struct {
	URI         string `json:"uri"`
	Description string `json:"description"`
}

type OrgMatrixMsc3488Asset struct {
	Type string `json:"type"`
}

type LiveLocation struct {
	//Body                  string                `json:"body"`
	Description           string                `json:"description"`
	Live                  bool                  `json:"live"`
	OrgMatrixMsc3488Asset OrgMatrixMsc3488Asset `json:"org.matrix.msc3488.asset"`
	OrgMatrixMsc3488Ts    int64                 `json:"org.matrix.msc3488.ts"`
	Timeout               int64                 `json:"timeout"`
}

type RelatedLocation struct {
	MRelatesTo               MRelatesTo               `json:"m.relates_to"`
	OrgMatrixMsc3488Location OrgMatrixMsc3488Location `json:"org.matrix.msc3488.location"`
	OrgMatrixMsc3488Ts       int64                    `json:"org.matrix.msc3488.ts"`
}

type MRelatesTo struct {
	EventID string `json:"event_id"`
	RelType string `json:"rel_type"`
}

type OrgMatrixMsc3488Location struct {
	URI string `json:"uri"`
}

func getGeoJsons() *geojson.FeatureCollection {
	file, err := ioutil.ReadFile("./points.geojson")
	if err != nil {
		log.Fatal("Error when opening file: ", err)
	}

	geoj, err := geojson.UnmarshalFeatureCollection(file)

	if err != nil {
		log.Fatal("Error during Unmarshal(): ", err)
	}

	return geoj
}

/*
*
Microservice for reporting live geolocations via the Matrix Protocol.
1. Check to see if an existing state event is running for the user for live locations
2. If yes, send the location via a m.relates_to message linking the location to the live state event.
3. If no, create and Send a state event, followed by #2.
*/
func main() {
	//fmt.Println("Logging into", *homeserver, "as", *username)
	readAccounts()
	//go launchService("tbxx", "uwj4fqw7bzt.UDK_ntu")
	//go launchService("gpsbot1", "!0h#x33TX5vW")
	/**
	This is hack but the main function is finishing before the launchServices can run
	*/
	for {
	}
}

func readAccounts() {
	file, err := os.Open("./accounts.txt")
	if err != nil {
		log.Fatal(err)
	}
	//defer file.Close()
	idx := 0

	scanner := bufio.NewScanner(file)
	// optionally, resize scanner's capacity for lines over 64K, see next example
	for scanner.Scan() {
		s := strings.Split(scanner.Text(), " ")
		//username := flag.String("username", s[0], "Matrix username localpart")
		//password := flag.String("password", s[1], "Matrix password")
		go launchService(s[0], s[1], idx*25)
		idx = idx + 1
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func launchService(username string, password string, startIdx int) {

	var LiveLocationBeaconInfo = event.NewEventType("org.matrix.msc3672.beacon_info")
	var LiveLocationBeacon = event.NewEventType("org.matrix.msc3672.beacon")

	now := time.Now()

	duration, _ := time.ParseDuration("2h30m")

	/**
	Not very familar with pointers, not sure if this is safe or not in goroutines
	*/
	client, err := mautrix.NewClient(*homeserver, "", "")

	if err != nil {
		panic(err)
	}
	_, err = client.Login(&mautrix.ReqLogin{
		Type:             "m.login.password",
		Identifier:       mautrix.UserIdentifier{Type: mautrix.IdentifierTypeUser, User: username},
		Password:         password,
		StoreCredentials: true,
	})

	if err != nil {
		panic(err)
	}

	fmt.Println("Login successful " + username)

	LiveLocationBeaconInfoContent := LiveLocation{
		Description:           "Live location hyu",
		Live:                  true,
		OrgMatrixMsc3488Asset: OrgMatrixMsc3488Asset{Type: "m.self"},
		OrgMatrixMsc3488Ts:    now.UnixMilli(),
		Timeout:               duration.Milliseconds(),
	}

	// Create a state event .. tell the room this user is about to start broadcasting a live gps stream.
	resp, err2 := client.SendStateEvent(id.RoomID(gpsTestRoom), LiveLocationBeaconInfo, fmt.Sprintf("@%s:%s", username, "matrix.org"), &LiveLocationBeaconInfoContent)

	if err2 != nil {
		panic(err2)
	}

	for i := startIdx; i < len(geos.Features); i++ {
		var loc = buildLocation(string(resp.EventID), geos.Features[i].Geometry)
		client.SendMessageEvent(id.RoomID(gpsTestRoom), LiveLocationBeacon, &loc)
		time.Sleep(1 * time.Second)

	}

	/**
	This is a blocking function
	*/
	err = client.Sync()

	if err != nil {
		panic(err)
	}
}

/*
*
Build a location event structure
*/
func buildLocation(eventId string, geometry *geojson.Geometry) RelatedLocation {
	var geoUri = fmt.Sprintf("geo:%f,%f;u=10", geometry.Point[1], geometry.Point[0])
	now := time.Now()

	return RelatedLocation{
		MRelatesTo:               MRelatesTo{EventID: eventId, RelType: "m.reference"},
		OrgMatrixMsc3488Location: OrgMatrixMsc3488Location{URI: geoUri},
		OrgMatrixMsc3488Ts:       now.UnixMilli(),
	}
}
