package config

import (
	"fmt"

	"github.com/hrthas-king/PulseBar/internal/watchlist"
)

func (c Config) HasMarketGroup(id string) bool {
	if ValidMarketGroup(id) {
		return true
	}
	for _, p := range c.CustomPairs {
		if p.ID() == id {
			return true
		}
	}
	return false
}

func (c Config) AddPair(pair watchlist.Pair) (Config, error) {
	p, err := watchlist.NewPair(pair.Code1, pair.Code2)
	if err != nil {
		return c, err
	}
	c.CustomPairs = append([]watchlist.Pair(nil), c.CustomPairs...)
	for _, existing := range c.CustomPairs {
		if existing == p {
			c.MarketGroup = p.ID()
			return c, nil
		}
	}
	c.CustomPairs = append(c.CustomPairs, p)
	c.MarketGroup = p.ID()
	return c, nil
}

func (c Config) RemovePair(id string) (Config, error) {
	var pairs []watchlist.Pair
	found := false
	for _, p := range c.CustomPairs {
		if p.ID() == id {
			found = true
		} else {
			pairs = append(pairs, p)
		}
	}
	if !found {
		return c, fmt.Errorf("该组合不存在")
	}
	c.CustomPairs = pairs
	if c.MarketGroup == id {
		c.MarketGroup = DefaultMarketGroup
	}
	return c, nil
}
