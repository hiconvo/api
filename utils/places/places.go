package places

import (
	"context"
	"os"
	"strings"

	"googlemaps.github.io/maps"

	"github.com/hiconvo/api/utils/secrets"
)

type Place struct {
	PlaceID string
	Address string
	Lat     float64
	Lng     float64
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

	fields = []maps.PlaceDetailsFieldMask{fieldPlaceID, fieldFormattedAddress, fieldGeometry}

}

func Resolve(ctx context.Context, placeID string) (Place, error) {
	result, err := client.PlaceDetails(ctx, &maps.PlaceDetailsRequest{
		PlaceID: placeID,
		Fields:  fields,
	})
	if err != nil {
		return Place{}, err
	}

	return Place{
		PlaceID: result.PlaceID,
		Address: result.FormattedAddress,
		Lat:     result.Geometry.Location.Lat,
		Lng:     result.Geometry.Location.Lng,
	}, nil
}
