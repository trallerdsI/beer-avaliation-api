package http

import "errors"

// detectImageType identifica o Content-Type de uma imagem pelos magic bytes
// (não pela extensão), mitigando upload de binários não-imagem renomeados.
// Apenas JPEG, PNG e WebP são aceites (allowlist do backend).
func detectImageType(data []byte) (string, error) {
	if len(data) < 4 {
		return "", errors.New("ficheiro demasiado pequeno")
	}
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg", nil
	case len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A:
		return "image/png", nil
	case len(data) >= 12 &&
		data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 && // RIFF
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50: // WEBP
		return "image/webp", nil
	}
	return "", errors.New("tipo de imagem não suportado")
}

// contentTypeToExt devolve a extensão (com ponto) para o Content-Type aceite.
func contentTypeToExt(ct string) string {
	switch ct {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}
