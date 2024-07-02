package resp

import "testing"

func TestReadCommands(t *testing.T) {
	cmds, lastbytes, err := ReadCommands([]byte("*3\r\n$3\r\nset\r\n$2\r\nk1\r\n$2\r\nv2\r\n"))
	_ = lastbytes
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cmds {
		for _, arg := range c.Args {
			t.Log(string(arg))
		}
	}
}
