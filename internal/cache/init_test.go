package cache

import (
	"context"
	"testing"

	"github.com/Circutor/edgex/pkg/models"
	"github.com/edgexfoundry/device-sdk-go/internal/common"
	"github.com/edgexfoundry/device-sdk-go/internal/mock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestInitCache(t *testing.T) {
	common.DeviceClient = &mock.DeviceClientMock{}
	InitCache()

	ctx := context.WithValue(context.Background(), common.CorrelationHeader, uuid.New().String())

	dsBeforeAddingToCache, _ := common.DeviceClient.DevicesForServiceByName(common.ServiceName, ctx)
	if dl := len(Devices().All()); dl != len(dsBeforeAddingToCache) {
		t.Errorf("the expected number of devices in cache is %d but got: %d:", len(dsBeforeAddingToCache), dl)
	}

	pMap := make(map[string]models.DeviceProfile, len(dsBeforeAddingToCache)*2)
	for _, d := range dsBeforeAddingToCache {
		pMap[d.Profile.Name] = d.Profile
	}
	if pl := len(Profiles().All()); pl != len(pMap) {
		t.Errorf("the expected number of device profiles in cache is %d but got: %d:", len(pMap), pl)
	} else {
		psFromCache := Profiles().All()
		for _, p := range psFromCache {
			assert.Equal(t, pMap[p.Name], p)
		}
	}
}
