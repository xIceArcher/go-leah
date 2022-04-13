package progress

type Bar interface {
	Add(int64)
	Set(int64)

	AddMax(int64)
	SetMax(int64)
}
