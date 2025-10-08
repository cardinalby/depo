package components

import (
	"github.com/cardinalby/depo"
)

type Components struct {
	A func() *AnyComponent
	B func() *AnyComponent
	C func() *AnyComponent
	D func() *AnyComponent
	E func() *AnyComponent
	F func() *AnyComponent
	G func() *AnyComponent
	H func() *AnyComponent
	I func() *AnyComponent
	J func() *AnyComponent
	K func() *AnyComponent
}

func GetComponents(reg *Registry) Components {
	var components Components

	components.A = depo.Provide(func() *AnyComponent {
		depo.UseLifecycle().
			AddReadinessRunnable(reg.NewRunnable(depo.UseComponentID(), "A")).Tag("A")
		return NewAnyComponent(
			components.D(),
		)
	})

	components.B = depo.Provide(func() *AnyComponent {
		depo.UseLifecycle().
			AddReadinessRunnable(reg.NewRunnable(depo.UseComponentID(), "B")).Tag("B")
		return NewAnyComponent(
			components.E(),
		)
	})

	components.C = depo.Provide(func() *AnyComponent {
		depo.UseLifecycle().
			AddReadinessRunnable(reg.NewRunnable(depo.UseComponentID(), "C")).Tag("C")
		return NewAnyComponent(
			components.F(),
		)
	})

	components.D = depo.Provide(func() *AnyComponent {
		depo.UseLifecycle().
			AddReadinessRunnable(reg.NewRunnable(depo.UseComponentID(), "D")).Tag("D")
		return NewAnyComponent(
			components.G(),
			components.F(),
		)
	})

	components.E = depo.Provide(func() *AnyComponent {
		depo.UseLifecycle().
			AddReadinessRunnable(reg.NewRunnable(depo.UseComponentID(), "E")).Tag("E")
		return NewAnyComponent(
			components.D(),
		)
	})

	components.F = depo.Provide(func() *AnyComponent {
		depo.UseLifecycle().
			AddReadinessRunnable(reg.NewRunnable(depo.UseComponentID(), "F")).Tag("F")
		return NewAnyComponent(
			components.G(),
			components.H(),
			components.I(),
		)
	})

	components.G = depo.Provide(func() *AnyComponent {
		depo.UseLifecycle().
			AddReadinessRunnable(reg.NewRunnable(depo.UseComponentID(), "G")).Tag("G")
		return NewAnyComponent(
			components.H(),
		)
	})

	components.H = depo.Provide(func() *AnyComponent {
		depo.UseLifecycle().
			AddReadinessRunnable(reg.NewRunnable(depo.UseComponentID(), "H")).Tag("H")
		return NewAnyComponent()
	})

	components.I = depo.Provide(func() *AnyComponent {
		depo.UseLifecycle().
			AddReadinessRunnable(reg.NewRunnable(depo.UseComponentID(), "I")).Tag("I")
		return NewAnyComponent()
	})

	components.J = depo.Provide(func() *AnyComponent {
		depo.UseLifecycle().
			AddReadinessRunnable(reg.NewRunnable(depo.UseComponentID(), "J")).Tag("J")
		return NewAnyComponent(
			components.K(),
		)
	})

	components.K = depo.Provide(func() *AnyComponent {
		depo.UseLifecycle().
			AddReadinessRunnable(reg.NewRunnable(depo.UseComponentID(), "K")).Tag("K")
		return NewAnyComponent()
	})

	return components
}

type AnyComponent struct {
}

func NewAnyComponent(deps ...any) *AnyComponent {
	return &AnyComponent{
		// would normally use deps to initialize the component
	}
}
