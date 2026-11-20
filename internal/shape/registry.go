package shape

// Registry maps endpoint names to their registered shapes. It is data, not
// policy: the shapes below are the ones the local gateway demo serves.
var Registry = map[string]*Shape{
	"account.create": {
		Name:     "account.create",
		MaxDepth: 3,
		Fields: map[string]Field{
			"username":     {Kind: KindString, Required: true, MaxBytes: 32, Class: ClassSafe},
			"display_name": {Kind: KindString, Required: true, MaxBytes: 24, Class: ClassText},
			"bio":          {Kind: KindString, MaxBytes: 64, Class: ClassText},
			"slug":         {Kind: KindString, MaxBytes: 16, Class: ClassSafe},
			"attempts":     {Kind: KindInt, Required: true},
			"enabled":      {Kind: KindBool, Required: true},
			"items":        {Kind: KindStringList, MaxBytes: 16, Class: ClassText},
			"owner.email":  {Kind: KindString, Required: true, MaxBytes: 64, Class: ClassSafe},
			"owner.name":   {Kind: KindString, MaxBytes: 32, Class: ClassText},
			"company":      {Kind: KindString, MaxBytes: 9, Class: ClassText},
		},
	},
	"event.ingest": {
		Name:     "event.ingest",
		MaxDepth: 3,
		Fields: map[string]Field{
			"event":   {Kind: KindString, Required: true, MaxBytes: 24, Class: ClassSafe},
			"payload": {Kind: KindString, MaxBytes: 48, Class: ClassText},
			"count":   {Kind: KindInt, Required: true},
			"dry_run": {Kind: KindBool},
		},
	},
	"profile.update": {
		Name:     "profile.update",
		MaxDepth: 2,
		Fields: map[string]Field{
			"nickname": {Kind: KindString, MaxBytes: 8, Class: ClassText},
			"note":     {Kind: KindString, MaxBytes: 24, Class: ClassText},
			"code":     {Kind: KindString, MaxBytes: 8, Class: ClassSafe},
		},
	},
}

// Get returns the shape registered under name.
func Get(name string) (*Shape, bool) {
	s, ok := Registry[name]
	return s, ok
}
