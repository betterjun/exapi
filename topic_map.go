package exapi

import (
	"sync"
)

// // 订单id到订单的映射
type TopicMap struct {
	//m    make(map[string]CurrencyPair),
	m sync.Map
}

func NewStreamMap() *TopicMap {
	return &TopicMap{}
}

func (m *TopicMap) Load(k string) (pair CurrencyPair, ok bool) {
	if val, ok := m.m.Load(k); !ok {
		return pair, false
	} else {
		pair, ok = val.(CurrencyPair)
		return pair, true
	}
}

func (m *TopicMap) LoadOrStore(k string, pair CurrencyPair) (actualPair CurrencyPair, loaded bool) {
	if val, ok := m.m.LoadOrStore(k, pair); !ok {
		actualPair = pair
	} else {
		actualPair, _ = val.(CurrencyPair)
		loaded = true
	}
	return
}

func (m *TopicMap) Store(k string, v CurrencyPair) {
	m.m.Store(k, v)
}

func (m *TopicMap) Delete(k string) {
	m.m.Delete(k)
}

func (m *TopicMap) Range(f func(k string, v CurrencyPair) bool) {
	m.m.Range(func(key, value interface{}) bool {
		k, _ := key.(string)
		v, _ := value.(CurrencyPair)
		return f(k, v)
	})
}
