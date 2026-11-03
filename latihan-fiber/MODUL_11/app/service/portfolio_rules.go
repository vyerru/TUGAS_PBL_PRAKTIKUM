package service

// NormalizeTechnologies merapikan daftar teknologi proyek: huruf kecil, tanpa
// spasi di tepi, tanpa duplikat, urutan pertama dipertahankan. Aturannya sama
// dengan NormalizeTags pada ulasan — tanpa normalisasi, "Go", "go", dan " go "
// terhitung tiga teknologi berbeda dan statistik teknologi menjadi tidak
// bermakna.
func NormalizeTechnologies(techs []string) []string {
	return NormalizeTags(techs)
}
