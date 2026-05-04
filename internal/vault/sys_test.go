package vault

import "testing"

func TestKVVersion(t *testing.T) {
	cases := []struct {
		name string
		mi   *MountInfo
		want int
	}{
		{"nil", nil, 0},
		{"non-kv", &MountInfo{Type: "pki"}, 0},
		{"kv default v1", &MountInfo{Type: "kv"}, 1},
		{"kv v1 explicit", &MountInfo{Type: "kv", Options: map[string]string{"version": "1"}}, 1},
		{"kv v2", &MountInfo{Type: "kv", Options: map[string]string{"version": "2"}}, 2},
		{"generic", &MountInfo{Type: "generic"}, 1},
		{"kv uppercase", &MountInfo{Type: "KV", Options: map[string]string{"version": "2"}}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := KVVersion(c.mi); got != c.want {
				t.Fatalf("KVVersion = %d want %d", got, c.want)
			}
			if got := IsKV(c.mi); got != (c.want > 0) {
				t.Fatalf("IsKV = %v want %v", got, c.want > 0)
			}
		})
	}
}
