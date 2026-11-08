package runtimecfg

import "testing"

func TestParseWeaponPreferences(t *testing.T) {
	for _, tc := range []struct {
		text string
		want WeaponPreferences
	}{
		{"", DefaultWeaponPreferences()},
		{" Shotgun, FIST ", WeaponPreferences{3, 1, 6, 9, 4, 2, 8, 5, 7}},
		{"bfg,rocket,plasma,ssg,chaingun,shotgun,pistol,chainsaw,fist", WeaponPreferences{7, 5, 6, 9, 4, 3, 2, 8, 1}},
	} {
		got, err := ParseWeaponPreferences(tc.text)
		if err != nil || got != tc.want {
			t.Fatalf("%q: got %v, %v; want %v", tc.text, got, err, tc.want)
		}
	}
	for _, text := range []string{"shotgun,Shotgun", "ssg,unknown", "shotgun,", "1", ","} {
		if _, err := ParseWeaponPreferences(text); err == nil {
			t.Fatalf("accepted invalid priority %q", text)
		}
	}
}
