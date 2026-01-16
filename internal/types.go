package internal

type LogLevel int8

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

func (l LogLevel) IsValid() bool {
	return l >= LevelDebug && l <= LevelFatal
}

type JSONFieldNames struct {
	Timestamp string
	Level     string
	Caller    string
	Message   string
	Fields    string
}

func (j *JSONFieldNames) IsComplete() bool {
	return j != nil &&
		j.Timestamp != "" &&
		j.Level != "" &&
		j.Caller != "" &&
		j.Message != "" &&
		j.Fields != ""
}

type JSONOptions struct {
	PrettyPrint bool
	Indent      string
	FieldNames  *JSONFieldNames
}
