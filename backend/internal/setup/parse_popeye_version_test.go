package setup

import "testing"

func TestParsePopeyeVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty input",
			in:   "",
			want: "",
		},
		{
			name: "plain version line",
			in:   "Version: 0.22.1",
			want: "0.22.1",
		},
		{
			name: "ANSI-colored banner with version, commit, date",
			in: "\x1b[38;5;122m ___ ___ _____ _____ \x1b[0m \x1b[38;5;75mK     .-'-. \x1b[0m\n" +
				"\x1b[38;5;122m| _ \\___| _ \\ __\\ \\ / / __|\x1b[0m \x1b[38;5;75m 8     __|  `\\ \x1b[0m\n" +
				"\x1b[38;5;122mVersion: \x1b[0m\x1b[38;5;15m0.22.1\x1b[0m\n" +
				"\x1b[38;5;122mCommit:  \x1b[0m\x1b[38;5;15mHomebrew\x1b[0m\n" +
				"\x1b[38;5;122mDate:    \x1b[0m\x1b[38;5;15m2025-01-28T15:23:31Z\x1b[0m (binary)",
			want: "0.22.1",
		},
		{
			name: "version line with leading whitespace",
			in:   "    Version:    1.2.3   \n",
			want: "1.2.3",
		},
		{
			name: "no version line falls back to first non-empty line",
			in:   "\n\nsome-tag-v1\nirrelevant\n",
			want: "some-tag-v1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePopeyeVersion(tc.in)
			if got != tc.want {
				t.Errorf("parsePopeyeVersion(...) = %q, want %q", got, tc.want)
			}
		})
	}
}
