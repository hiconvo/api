package places

import (
	"context"

	"googlemaps.github.io/maps"
)

type testClient struct{}

func (c *testClient) PlaceDetails(ctx context.Context, r *maps.PlaceDetailsRequest) (maps.PlaceDetailsResult, error) {
	return maps.PlaceDetailsResult{
		PlaceID:          "place_id",
		FormattedAddress: "formatted_address",
		Geometry: maps.AddressGeometry{
			Location: maps.LatLng{
				Lat: 0.0,
				Lng: 0.0,
			},
		},
	}, nil
}
