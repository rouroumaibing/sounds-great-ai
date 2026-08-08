package a2a

import "testing"

func TestMethodConstants_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "TasksSend", got: MethodTasksSend, want: "tasks/send"},
		{name: "TasksGet", got: MethodTasksGet, want: "tasks/get"},
		{name: "TasksCancel", got: MethodTasksCancel, want: "tasks/cancel"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}
