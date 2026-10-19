// Package web, derlenmiş arayüz dosyalarını (npm run build) Go programının içine
// gömer; sunucu tek dosya olarak dağıtılır.
package web

import "embed"

// dist her zaman .gitkeep içerir; arayüz derlenmeden de Go derlenebilsin.
//
//go:embed all:dist
var dist embed.FS

// Tracker derlenmiş takip betiğini döndürür; derlenmemişse nil.
func Tracker() []byte {
	b, err := dist.ReadFile("dist/nabiz.js")
	if err != nil {
		return nil
	}
	return b
}
