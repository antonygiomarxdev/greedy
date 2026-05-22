package strategy

func RegisterAll(r *Registry) {
	r.Register(&DCABuilder{})
	r.Register(&GridBuilder{})
	r.Register(&SignalBuilder{})
}
