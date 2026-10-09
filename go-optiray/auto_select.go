package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

type serverProbe func(context.Context, Object, string) int
type candidateScore struct {
	server         Object
	latency, index int
}

func pingBadge(minimum, median, maximum int) Object {
	color := "ping-green"
	if median >= 800 {
		color = "ping-red"
	} else if median >= 400 {
		color = "ping-orange"
	}
	return Object{"text": fmt.Sprintf("%d ms", median), "cls": color, "min": minimum, "max": maximum}
}
func chooseServer(ctx context.Context, d Object, subID int, probe serverProbe) (Object, string, Object, error) {
	var sub Object
	for _, v := range list(d["subscriptions"]) {
		if integer(obj(v)["id"]) == subID {
			sub = obj(v)
			break
		}
	}
	pings := Object{}
	if sub == nil {
		return nil, "", pings, fmt.Errorf("Подписка не найдена")
	}
	favorites := map[string]bool{}
	for _, v := range list(d["favorites"]) {
		favorites[str(v)] = true
	}
	servers := []Object{}
	for _, v := range list(sub["servers"]) {
		s := obj(v)
		if str(s["sid"]) == "" {
			continue
		}
		if str(d["auto_select_source"]) == "favorites" && !favorites[str(s["sid"])] {
			continue
		}
		if supportedServer(s) {
			servers = append(servers, s)
		}
	}
	if len(servers) == 0 {
		return nil, "", pings, fmt.Errorf("Нет пригодных серверов для автовыбора")
	}
	scores := make([]int, len(servers))
	for i := range scores {
		scores[i] = -1
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < 5; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if ctx.Err() == nil {
					scores[i] = probe(ctx, servers[i], "tcp")
				}
			}
		}()
	}
	for i := range servers {
		select {
		case jobs <- i:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return nil, "", pings, ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()
	if ctx.Err() != nil {
		return nil, "", pings, ctx.Err()
	}
	alive := []candidateScore{}
	quic := []candidateScore{}
	for i, s := range servers {
		id := str(s["sid"])
		lat := scores[i]
		if lat >= 0 {
			alive = append(alive, candidateScore{s, lat, i})
			pings[id] = pingBadge(lat, lat, lat)
		} else if lat == -2 {
			quic = append(quic, candidateScore{s, -1, i})
			pings[id] = Object{"text": "-- ms", "cls": "", "min": 0, "max": 0}
		} else {
			pings[id] = Object{"text": "dead", "cls": "ping-red", "min": 0, "max": 0}
		}
	}
	sort.SliceStable(alive, func(i, j int) bool { return alive[i].latency < alive[j].latency })
	shortlist := append([]candidateScore{}, quic...)
	limit := len(alive)
	if limit > 10 {
		limit = 10
	}
	shortlist = append(shortlist, alive[:limit]...)
	var best Object
	bestLatency, bestIndex := int(^uint(0)>>1), int(^uint(0)>>1)
	for _, candidate := range shortlist {
		samples := []int{}
		for sample := 0; sample < 3; sample++ {
			if ctx.Err() != nil {
				return nil, "", pings, ctx.Err()
			}
			if lat := probe(ctx, candidate.server, "real"); lat >= 0 {
				samples = append(samples, lat)
			}
		}
		if len(samples) == 0 {
			continue
		}
		sort.Ints(samples)
		median := samples[len(samples)/2]
		pings[str(candidate.server["sid"])] = pingBadge(samples[0], median, samples[len(samples)-1])
		if median < bestLatency || median == bestLatency && candidate.index < bestIndex {
			best = candidate.server
			bestLatency = median
			bestIndex = candidate.index
		}
	}
	if best != nil {
		return best, "", pings, nil
	}
	if len(alive) > 0 {
		return alive[0].server, "HTTP-проверка не прошла, выбран лучший по TCP", pings, nil
	}
	return nil, "", pings, fmt.Errorf("Не удалось найти рабочий сервер")
}
func (a *App) connectSelection(ctx context.Context, sid, name string) (Object, error) {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if a.core == nil {
		return nil, fmt.Errorf("Запуск ядра отключён")
	}
	d := a.store.snapshot()
	if !optirayEnabled(d) {
		return nil, fmt.Errorf("OptiRay выключен. Включите переключатель на главной странице")
	}
	server, _ := findServer(d, sid, name)
	if server == nil {
		return nil, fmt.Errorf("server not found")
	}
	port := integer(d["proxy_port"])
	e := a.core.ApplyServer(server, d, port)
	if e != nil {
		return nil, e
	}
	warning := "Настройка прокси Keenetic отключена для этой тестовой установки"
	var undo func() error
	if a.keenetic != nil {
		undo, e = a.keenetic.Apply(ctx, str(d["proxy_name"]), str(d["proxy_host"]), port)
		if e != nil {
			warning = "Ядро запущено, но прокси Keenetic не настроен: " + e.Error()
		} else {
			warning = ""
		}
	}
	if e = a.store.change(func(current Object) error {
		s, _ := findServer(current, str(server["sid"]), "")
		if s == nil {
			return fmt.Errorf("subscription changed during connection")
		}
		current["selected"] = str(server["sid"])
		return nil
	}); e != nil {
		if undo != nil {
			if rollbackError := undo(); rollbackError != nil {
				a.core.Stop()
				return nil, fmt.Errorf("%v; Keenetic rollback failed", e)
			}
		}
		if rollback := a.rollbackRuntime(d, nil); rollback != nil {
			return nil, fmt.Errorf("connection save failed: %v; rollback failed: %v", e, rollback)
		}
		return nil, e
	}
	verified := warmupProxy(ctx, port, first(str(d["test_button_url"]), str(d["test_url"])))
	return Object{"status": "ok", "sid": str(server["sid"]), "name": str(server["name"]), "verified": verified, "warning": warning}, nil
}
func (a *App) autoSelect(ctx context.Context, subID int) (Object, error) {
	if !optirayEnabled(a.store.snapshot()) {
		return nil, fmt.Errorf("OptiRay выключен")
	}
	d := a.store.snapshot()
	seconds, _ := time.ParseDuration(str(d["ping_timeout"]) + "s")
	if seconds < 500*time.Millisecond || seconds > 30*time.Second {
		seconds = 3 * time.Second
	}
	samples := map[string][]int{}
	best, warning, pings, e := chooseServer(ctx, d, subID, func(ctx context.Context, s Object, kind string) int {
		if kind == "tcp" {
			return a.ping(ctx, d, s, kind, seconds)
		}
		id := str(s["sid"])
		values, ok := samples[id]
		if !ok {
			values = a.realSamples(ctx, d, s, seconds)
		}
		if len(values) == 0 {
			return -1
		}
		value := values[0]
		samples[id] = values[1:]
		return value
	})
	if e != nil {
		return Object{"pings": pings}, e
	}
	current := a.store.snapshot()
	_, sub := findServer(current, str(best["sid"]), "")
	if sub == nil || integer(sub["id"]) != subID {
		return Object{"pings": pings}, fmt.Errorf("Подписка изменилась во время проверки")
	}
	result, e := a.connectSelection(ctx, str(best["sid"]), "")
	if e != nil {
		return Object{"pings": pings}, e
	}
	result["pings"] = pings
	if warning != "" {
		result["warning"] = warning + "; " + str(result["warning"])
	}
	return result, nil
}
