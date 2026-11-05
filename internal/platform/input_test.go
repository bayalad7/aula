package platform

import "testing"

func TestKeyCodesAreDistinct(t *testing.T) {
	seen := map[keyCode]string{}
	for code, k := range keyCodes {
		if k.scan == 0 && k.vk == 0 {
			t.Errorf("%s has neither a scancode nor a virtual key", code)
		}
		if k.scan != 0 && k.vk != 0 {
			t.Errorf("%s has both a scancode and a virtual key", code)
		}
		if other, dup := seen[k]; dup {
			t.Errorf("%s and %s inject the same key", code, other)
		}
		seen[k] = code
	}
	if !KnownKey("KeyA") || KnownKey("MetaLeft") {
		t.Fatal("KnownKey misbehaves")
	}
}
