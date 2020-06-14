package builder

import (
	"github.com/betterjun/exapi"
	"github.com/stretchr/testify/assert"
	"testing"
)

var builder = NewAPIBuilder()

func TestAPIBuilder_BuildSpot(t *testing.T) {
	assert.Equal(t, builder.APIKey("").APISecretkey("").BuildSpot(exapi.HUOBI).GetExchangeName(), exapi.HUOBI)
	assert.Equal(t, builder.APIKey("").APISecretkey("").BuildSpot(exapi.BINANCE).GetExchangeName(), exapi.BINANCE)
	assert.Equal(t, builder.APIKey("").APISecretkey("").BuildSpot(exapi.OKEX).GetExchangeName(), exapi.OKEX)
	assert.Equal(t, builder.APIKey("").APISecretkey("").BuildSpot(exapi.ZB).GetExchangeName(), exapi.ZB)
	assert.Equal(t, builder.APIKey("").APISecretkey("").BuildSpot(exapi.GATE).GetExchangeName(), exapi.GATE)
}

func TestAPIBuilder_BuildSpotWebsocket(t *testing.T) {
	//builder.APIKey("").APISecretkey("").BuildSpotWebsocket(exapi.HUOBI, "")
	//builder.APIKey("").APISecretkey("").BuildSpotWebsocket(exapi.BINANCE, "")
	//builder.APIKey("").APISecretkey("").BuildSpotWebsocket(exapi.OKEX, "")
	//builder.APIKey("").APISecretkey("").BuildSpotWebsocket(exapi.ZB, "")
	//builder.APIKey("").APISecretkey("").BuildSpotWebsocket(exapi.GATE, "")
}
