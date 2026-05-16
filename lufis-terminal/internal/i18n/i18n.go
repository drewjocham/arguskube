package i18n

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Lang string

const (
	EN Lang = "en"
	DE Lang = "de"
	FR Lang = "fr"
	ES Lang = "es"
	JA Lang = "ja"
	ZH Lang = "zh"
)

type Bundle struct {
	mu   sync.RWMutex
	lang Lang
	msgs map[string]string
}

func NewBundle(lang Lang) *Bundle {
	b := &Bundle{lang: lang, msgs: make(map[string]string)}
	b.loadDefaults()
	return b
}

func (b *Bundle) SetLang(lang Lang) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lang = lang
	b.msgs = make(map[string]string)
	b.loadDefaults()
}

func (b *Bundle) T(key string, args ...interface{}) string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	msg, ok := b.msgs[key]
	if !ok {
		return key
	}
	return msg
}

func (b *Bundle) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var msgs map[string]string
	if err := json.Unmarshal(data, &msgs); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for k, v := range msgs {
		b.msgs[k] = v
	}
	return nil
}

func (b *Bundle) Lang() Lang { return b.lang }

func (b *Bundle) loadDefaults() {
	b.msgs["terminal.title"] = "Argus Terminal"
	b.msgs["menu.file"] = "File"
	b.msgs["menu.edit"] = "Edit"
	b.msgs["menu.view"] = "View"
	b.msgs["menu.help"] = "Help"
	b.msgs["status.recording"] = "Recording"
	b.msgs["status.connected"] = "Connected"
	b.msgs["status.disconnected"] = "Disconnected"
	b.msgs["prompt.search"] = "Search..."
	b.msgs["prompt.command"] = "Command..."
	b.msgs["error.timeout"] = "Operation timed out"
	b.msgs["error.connection"] = "Connection failed"
	b.msgs["dialog.confirm"] = "Confirm"
	b.msgs["dialog.cancel"] = "Cancel"
	b.msgs["pane.close"] = "Close"
	b.msgs["pane.split"] = "Split"
	b.msgs["note.title"] = "Notes"

	switch b.lang {
	case DE:
		b.msgs["menu.file"] = "Datei"
		b.msgs["menu.edit"] = "Bearbeiten"
		b.msgs["menu.view"] = "Ansicht"
		b.msgs["menu.help"] = "Hilfe"
		b.msgs["dialog.confirm"] = "Bestätigen"
		b.msgs["dialog.cancel"] = "Abbrechen"
	case FR:
		b.msgs["menu.file"] = "Fichier"
		b.msgs["menu.edit"] = "Éditer"
		b.msgs["menu.view"] = "Affichage"
		b.msgs["menu.help"] = "Aide"
		b.msgs["dialog.confirm"] = "Confirmer"
		b.msgs["dialog.cancel"] = "Annuler"
	case ES:
		b.msgs["menu.file"] = "Archivo"
		b.msgs["menu.edit"] = "Editar"
		b.msgs["menu.view"] = "Ver"
		b.msgs["menu.help"] = "Ayuda"
	case JA:
		b.msgs["menu.file"] = "ファイル"
		b.msgs["menu.edit"] = "編集"
		b.msgs["menu.view"] = "表示"
		b.msgs["menu.help"] = "ヘルプ"
	}
}

func DetectLang() Lang {
	lang := os.Getenv("LANG")
	if len(lang) >= 2 {
		switch lang[:2] {
		case "de":
			return DE
		case "fr":
			return FR
		case "es":
			return ES
		case "ja":
			return JA
		case "zh":
			return ZH
		}
	}
	return EN
}

func LoadFromDir(dir string) (map[Lang]*Bundle, error) {
	bundles := make(map[Lang]*Bundle)
	for _, lang := range []Lang{EN, DE, FR, ES, JA, ZH} {
		b := NewBundle(lang)
		path := filepath.Join(dir, string(lang)+".json")
		if data, err := os.ReadFile(path); err == nil {
			var msgs map[string]string
			if json.Unmarshal(data, &msgs) == nil {
				for k, v := range msgs {
					_ = b
					_ = k
					_ = v
				}
			}
		}
		bundles[lang] = b
	}
	return bundles, nil
}
