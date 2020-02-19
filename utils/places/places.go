package places

import (
	"context"
	"net/http"
	"os"
	"strings"

	"googlemaps.github.io/maps"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/utils/secrets"
)

type Place struct {
	PlaceID   string
	Address   string
	Lat       float64
	Lng       float64
	UTCOffset int
}

type resolver interface {
	PlaceDetails(ctx context.Context, r *maps.PlaceDetailsRequest) (maps.PlaceDetailsResult, error)
}

var client resolver
var fields []maps.PlaceDetailsFieldMask

func init() {
	if strings.HasSuffix(os.Args[0], ".test") {
		client = &testClient{}
	} else {
		c, err := maps.NewClient(maps.WithAPIKey(secrets.Get("GOOGLE_MAPS_API_KEY", "")))
		if err != nil {
			panic(err)
		} else {
			client = c
		}
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

	fields = []maps.PlaceDetailsFieldMask{
		fieldPlaceID,
		fieldName,
		fieldFormattedAddress,
		fieldGeometry,
		fieldUTCOffset,
	}
}

func Resolve(ctx context.Context, placeID string) (Place, error) {
	result, err := client.PlaceDetails(ctx, &maps.PlaceDetailsRequest{
		PlaceID: placeID,
		Fields:  fields,
	})
	if err != nil {
		return Place{}, errors.E(errors.Op("places.Resolve"),
			map[string]string{"placeId": "Could not resolve place"},
			http.StatusBadRequest,
			err)
	}

	address := strings.Join([]string{result.Name, result.FormattedAddress}, ", ")

	return Place{
		PlaceID:   result.PlaceID,
		Address:   address,
		Lat:       result.Geometry.Location.Lat,
		Lng:       result.Geometry.Location.Lng,
		UTCOffset: *result.UTCOffset * 60,
	}, nil
}
