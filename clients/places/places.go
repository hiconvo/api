package places

import (
	"context"
	"net/http"
	"strings"

	"googlemaps.github.io/maps"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/log"
)

type Place struct {
	PlaceID   string
	Address   string
	Lat       float64
	Lng       float64
	UTCOffset int
}

type Client interface {
	Resolve(ctx context.Context, placeID string, userUTCOffset int) (Place, error)
}

type clientImpl struct {
	client             *maps.Client
	fields             []maps.PlaceDetailsFieldMask
	voiceCallLocations []Place
}

func NewClient(apiKey string) Client {
	c, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		panic(errors.E(errors.Op("places.NewClient"), err))
	}

	fieldName, err := maps.ParsePlaceDetailsFieldMask("name")
	if err != nil {
		panic(err)
	}

	fieldPlaceID, err := maps.ParsePlaceDetailsFieldMask("place_id")
	if err != nil {
		panic(err)
	}

	fieldFormattedAddress, err := maps.ParsePlaceDetailsFieldMask("formatted_address")
	if err != nil {
		panic(err)
	}

	fieldGeometry, err := maps.ParsePlaceDetailsFieldMask("geometry")
	if err != nil {
		panic(err)
	}

	fieldUTCOffset, err := maps.ParsePlaceDetailsFieldMask("utc_offset")
	if err != nil {
		panic(err)
	}

	return &clientImpl{
		client: c,
		fields: []maps.PlaceDetailsFieldMask{
			fieldPlaceID,
			fieldName,
			fieldFormattedAddress,
			fieldGeometry,
			fieldUTCOffset,
		},
		voiceCallLocations: []Place{
			{
				PlaceID: "whatsapp",
				Address: "WhatsApp",
			},
			{
				PlaceID: "facetime",
				Address: "FaceTime",
			},
			{
				PlaceID: "googlehangouts",
				Address: "Google Hangouts",
			},
			{
				PlaceID: "zoom",
				Address: "Zoom",
			},
			{
				PlaceID: "skype",
				Address: "Skype",
			},
			{
				PlaceID: "other",
				Address: "Other",
			},
		},
	}
}

func (c *clientImpl) Resolve(ctx context.Context, placeID string, userUTCOffset int) (Place, error) {
	for i := range c.voiceCallLocations {
		if c.voiceCallLocations[i].PlaceID == placeID {
			pl := c.voiceCallLocations[i]
			pl.UTCOffset = userUTCOffset
			return pl, nil
		}
	}

	result, err := c.client.PlaceDetails(ctx, &maps.PlaceDetailsRequest{
		PlaceID: placeID,
		Fields:  c.fields,
	})
	if err != nil {
		return Place{}, errors.E(errors.Op("places.Resolve"),
			map[string]string{"placeId": "Could not resolve place"},
			http.StatusBadRequest,
			err)
	}

	var address string
	if strings.HasPrefix(result.FormattedAddress, result.Name) {
		address = result.FormattedAddress
	} else {
		address = strings.Join([]string{result.Name, result.FormattedAddress}, ", ")
	}

	return Place{
		PlaceID:   result.PlaceID,
		Address:   address,
		Lat:       result.Geometry.Location.Lat,
		Lng:       result.Geometry.Location.Lng,
		UTCOffset: *result.UTCOffset * 60,
	}, nil
}

type loggerImpl struct{}

func NewLogger() Client {
	log.Print("places.NewLogger: USING PLACES LOGGER FOR LOCAL DEVELOPMENT")
	return &loggerImpl{}
}

func (l *loggerImpl) Resolve(ctx context.Context, placeID string, userUTCOffset int) (Place, error) {
	log.Printf("places.Resolve(placeID=%s)", placeID)

	return Place{
		PlaceID:   "0123456789",
		Address:   "1 Infinite Loop",
		Lat:       0.0,
		Lng:       0.0,
		UTCOffset: 0,
	}, nil
}
