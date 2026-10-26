package main

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
)

var geoNames = []string{"geoip.dat", "geosite.dat"}

func validGeoName(name string) bool { return name == "geoip.dat" || name == "geosite.dat" }

func geoURL(name, address string) (string, error) {
	address = strings.TrimSpace(address)
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return "", fmt.Errorf("%s: укажите прямую HTTPS-ссылку на DAT-файл", name)
	}
	file := strings.ToLower(path.Base(u.Path))
	if (file == "geoip.dat" || file == "geosite.dat") && file != name {
		return "", fmt.Errorf("Для %s указана ссылка на %s", name, file)
	}
	return u.String(), nil
}

// Decode protobuf wire fields without interpreting one database as the other.
func geoFields(data []byte, visit func(uint64, uint64, []byte, uint64) error) error {
	for len(data) > 0 {
		tag, n := binary.Uvarint(data)
		if n <= 0 || tag>>3 == 0 {
			return fmt.Errorf("Повреждённый DAT")
		}
		data = data[n:]
		var value []byte
		var number uint64
		switch tag & 7 {
		case 0:
			number, n = binary.Uvarint(data)
			if n <= 0 {
				return fmt.Errorf("Повреждённый DAT")
			}
			data = data[n:]
		case 1:
			if len(data) < 8 {
				return fmt.Errorf("Повреждённый DAT")
			}
			value = data[:8]
			data = data[8:]
		case 2:
			size, n := binary.Uvarint(data)
			if n <= 0 {
				return fmt.Errorf("Повреждённый DAT")
			}
			data = data[n:]
			if size > uint64(len(data)) {
				return fmt.Errorf("Повреждённый DAT")
			}
			value = data[:int(size)]
			data = data[int(size):]
		case 5:
			if len(data) < 4 {
				return fmt.Errorf("Повреждённый DAT")
			}
			value = data[:4]
			data = data[4:]
		default:
			return fmt.Errorf("Некорректный формат DAT")
		}
		if err := visit(tag>>3, tag&7, value, number); err != nil {
			return err
		}
	}
	return nil
}

func validateGeoRecord(data []byte, name string) error {
	var ip []byte
	var prefix uint64
	domain := ""
	err := geoFields(data, func(field, wire uint64, value []byte, number uint64) error {
		if name == "geoip.dat" {
			if field == 1 {
				if wire != 2 {
					return fmt.Errorf("Ожидался GeoIP, получен GeoSite")
				}
				ip = value
			}
			if field == 2 {
				if wire != 0 {
					return fmt.Errorf("Ожидался GeoIP, получен GeoSite")
				}
				prefix = number
			}
		} else {
			if field == 1 && (wire != 0 || number > 3) {
				return fmt.Errorf("Ожидался GeoSite, получен GeoIP")
			}
			if field == 2 {
				if wire != 2 || !utf8.Valid(value) {
					return fmt.Errorf("Некорректный домен GeoSite")
				}
				domain = string(value)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if name == "geoip.dat" {
		if (len(ip) != 4 && len(ip) != 16) || prefix > uint64(len(ip)*8) {
			return fmt.Errorf("Некорректная IP-сеть GeoIP")
		}
	} else if domain == "" {
		return fmt.Errorf("В GeoSite отсутствует домен")
	}
	return nil
}

func validateGeoData(data []byte, name string) error {
	if !validGeoName(name) {
		return fmt.Errorf("Неизвестный DAT-файл")
	}
	entries, records := 0, 0
	err := geoFields(data, func(field, wire uint64, value []byte, _ uint64) error {
		if field != 1 || wire != 2 {
			return fmt.Errorf("Некорректная база GeoIP / GeoSite")
		}
		code := false
		err := geoFields(value, func(field, wire uint64, value []byte, _ uint64) error {
			if field == 1 {
				if wire != 2 || len(value) == 0 || !utf8.Valid(value) {
					return fmt.Errorf("Некорректный код категории DAT")
				}
				code = true
			}
			if field == 2 {
				if wire != 2 {
					return fmt.Errorf("Некорректная запись DAT")
				}
				if err := validateGeoRecord(value, name); err != nil {
					return err
				}
				records++
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !code {
			return fmt.Errorf("В DAT отсутствует код категории")
		}
		entries++
		return nil
	})
	if err != nil {
		return err
	}
	if entries == 0 || records == 0 {
		return fmt.Errorf("DAT не содержит правил нужного типа")
	}
	return nil
}

func geoFileConfig(d Object, name string) Object {
	if cfg := obj(obj(d["geo_data_files"])[name]); len(cfg) > 0 {
		return cfg
	}
	if str(d["geo_data_source"]) == "github" && str(d["geo_data_repository"]) != "" {
		return Object{"source": "github", "url": "https://github.com/" + str(d["geo_data_repository"]) + "/releases/latest/download/" + name}
	}
	return Object{"source": "standard", "url": ""}
}

func (a *App) geoInfo() Object {
	d := a.store.snapshot()
	source := "standard"
	files := []Object{}
	for _, name := range geoNames {
		path := filepath.Join(a.store.dir, "bin", name)
		cfg := geoFileConfig(d, name)
		if str(cfg["source"]) == "github" {
			source = "github"
		}
		item := Object{"name": name, "path": path, "exists": false, "source": cfg["source"], "url": cfg["url"]}
		if stat, err := os.Stat(path); err == nil && stat.Mode().IsRegular() {
			item["exists"] = true
			item["size"] = stat.Size()
			item["updated_at"] = stat.ModTime().UTC().Format(time.RFC3339)
		}
		files = append(files, item)
	}
	return Object{"ok": true, "source": source, "files": files}
}

func (a *App) updateGeo(ctx context.Context, source, name, address string) error {
	return a.updateGeoFiles(ctx, source, map[string]string{name: address})
}
func (a *App) updateGeoFiles(ctx context.Context, source string, requested map[string]string) error {
	if source != "standard" && source != "github" {
		return fmt.Errorf("Неизвестный источник")
	}
	names := []string{}
	addresses := map[string]string{}
	for name := range requested {
		if !validGeoName(name) {
			return fmt.Errorf("Неизвестный DAT-файл")
		}
	}
	for _, name := range geoNames {
		address, ok := requested[name]
		if !ok {
			continue
		}
		if source == "github" {
			var err error
			address, err = geoURL(name, address)
			if err != nil {
				return err
			}
		} else {
			address = ""
		}
		names = append(names, name)
		addresses[name] = address
	}
	if len(names) == 0 {
		return fmt.Errorf("Не выбраны DAT-файлы")
	}
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	if source == "github" {
		seen := map[string]bool{}
		for _, name := range geoNames {
			address, ok := addresses[name]
			if !ok {
				address = str(geoFileConfig(a.store.snapshot(), name)["url"])
			}
			if address != "" {
				if seen[address] {
					return fmt.Errorf("Для GeoIP и GeoSite нужны разные ссылки")
				}
				seen[address] = true
			}
		}
	}
	var err error
	root := filepath.Join(a.store.dir, "bin")
	if err = os.MkdirAll(root, 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(root, ".geo-update-")
	if err != nil {
		return err
	}
	keepStage := false
	defer func() {
		if !keepStage {
			_ = os.RemoveAll(stage)
		}
	}()
	if source == "github" {
		for _, name := range names {
			if err = downloadCore(ctx, addresses[name], filepath.Join(stage, name)); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	} else {
		arch, err := coreArchitecture("xray", runtime.GOARCH)
		if err != nil {
			return err
		}
		archivePath := filepath.Join(stage, "xray.zip")
		if err = downloadCore(ctx, "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-"+arch+".zip", archivePath); err != nil {
			return err
		}
		archive, err := zip.OpenReader(archivePath)
		if err != nil {
			return err
		}
		defer archive.Close()
		for _, file := range archive.File {
			if _, wanted := addresses[file.Name]; !wanted {
				continue
			}
			if file.UncompressedSize64 > 128<<20 {
				return fmt.Errorf("DAT слишком большой")
			}
			reader, err := file.Open()
			if err != nil {
				return err
			}
			target, err := os.OpenFile(filepath.Join(stage, file.Name), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				reader.Close()
				return err
			}
			_, err = io.Copy(target, io.LimitReader(reader, (128<<20)+1))
			reader.Close()
			closeErr := target.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(stage, name))
		if err != nil {
			return fmt.Errorf("%s: файл отсутствует: %w", name, err)
		}
		if err = validateGeoData(data, name); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	undo, err := replaceCoreFiles(root, stage, names)
	if err != nil {
		return err
	}
	d := a.store.snapshot()
	active := a.core != nil && a.core.Running() && first(a.core.kind, "xray") == "xray"
	restore := func(cause error) error {
		if err := undo(); err != nil {
			keepStage = true
			return fmt.Errorf("%v; ошибка отката файлов: %w; копия: %s", cause, err, stage)
		}
		if active {
			if err := a.rollbackRuntime(d, nil); err != nil {
				return fmt.Errorf("%v; ошибка восстановления соединения: %w", cause, err)
			}
		}
		return cause
	}
	if active {
		server, _ := findServer(d, str(d["selected"]), "")
		if server == nil {
			return restore(fmt.Errorf("Активный сервер не найден"))
		}
		if err = a.core.ApplyServer(server, d, integer(d["proxy_port"])); err != nil {
			return restore(err)
		}
	}
	if err = a.store.change(func(next Object) error {
		configs := obj(next["geo_data_files"])
		if len(configs) == 0 {
			configs = Object{}
			for _, file := range geoNames {
				configs[file] = geoFileConfig(next, file)
			}
		}
		for _, name := range names {
			address := addresses[name]
			if source == "standard" {
				address = str(geoFileConfig(next, name)["url"])
				if entered := strings.TrimSpace(requested[name]); entered != "" {
					address = entered
				}
			}
			configs[name] = Object{"source": source, "url": address}
		}
		next["geo_data_files"] = configs
		delete(next, "geo_data_source")
		delete(next, "geo_data_repository")
		return nil
	}); err != nil {
		return restore(err)
	}
	return nil
}

func (a *App) geoAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		jsonReply(w, 200, a.geoInfo())
		return
	}
	if r.Method != "POST" {
		jsonReply(w, 405, Object{"ok": false, "error": "POST required"})
		return
	}
	if err := r.ParseForm(); err != nil {
		jsonReply(w, 400, Object{"ok": false, "error": err.Error()})
		return
	}
	requested := map[string]string{}
	if name := r.Form.Get("name"); name != "" {
		requested[name] = r.Form.Get("url")
	} else {
		requested["geoip.dat"] = r.Form.Get("geoip_url")
		requested["geosite.dat"] = r.Form.Get("geosite_url")
	}
	if err := a.updateGeoFiles(r.Context(), r.Form.Get("source"), requested); err != nil {
		jsonReply(w, 400, Object{"ok": false, "error": err.Error()})
		return
	}
	jsonReply(w, 200, a.geoInfo())
}
