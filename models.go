package main

type baseModel struct {
	Id        int     `db:"id"`
	CreatedAt *string `db:"createdAt"`
	UpdatedAt *string `db:"updatedAt"`
}

type v2Hotspot struct {
	baseModel

	ScreenId          *int    `db:"screenId"`
	HotspotTemplateId *int    `db:"hotspotTemplateId"`
	Type              string  `db:"type"`
	Left              float64 `db:"left"`
	Top               float64 `db:"top"`
	Width             float64 `db:"width"`
	Height            float64 `db:"height"`
	Settings          *string `db:"settings"`
}

type v2HotspotTemplate struct {
	baseModel

	PrototypeId int    `db:"prototypeId"`
	Title       string `db:"title"`
}

type v2HotspotTemplateScreenRel struct {
	baseModel

	HotspotTemplateId int `db:"hotspotTemplateId"`
	ScreenId          int `db:"screenId"`
}

type v2Project struct {
	baseModel

	Title    string `db:"title"`
	Archived *int   `db:"archived"`
}

type v2ProjectLink struct {
	baseModel

	ProjectId      int     `db:"projectId"`
	Slug           string  `db:"slug"`
	PasswordHash   *string `db:"passwordHash"`
	AllowComments  *int    `db:"allowComments"`
	AllowGuideline *int    `db:"allowGuideline"`
}

type v2ProjectLinkPrototypeRel struct {
	baseModel

	ProjectLinkId int `db:"projectLinkId"`
	PrototypeId   int `db:"prototypeId"`
}

type v2Prototype struct {
	baseModel

	ProjectId   int     `db:"projectId"`
	Title       string  `db:"title"`
	Type        string  `db:"type"`
	Width       float64 `db:"width"`
	Height      float64 `db:"height"`
	ScaleFactor float64 `db:"scaleFactor"`
}

type v2Screen struct {
	baseModel

	PrototypeId int     `db:"prototypeId"`
	Order       int     `db:"order"`
	Title       string  `db:"title"`
	Alignment   string  `db:"alignment"`
	Background  string  `db:"background"`
	FixedHeader float64 `db:"fixedHeader"`
	FixedFooter float64 `db:"fixedFooter"`
	FilePath    string  `db:"filePath"`
}

type v2ScreenComment struct {
	baseModel

	ReplyTo  *int    `db:"replyTo"`
	ScreenId int     `db:"screenId"`
	From     string  `db:"from"`
	Message  string  `db:"message"`
	Left     float64 `db:"left"`
	Top      float64 `db:"top"`
	Status   string  `db:"status"`
}

type v2User struct {
	baseModel

	Type               string  `db:"type"`
	Email              string  `db:"email"`
	PasswordHash       string  `db:"passwordHash"`
	PasswordResetToken *string `db:"passwordResetToken"`
	AuthKey            string  `db:"authKey"`
	FirstName          *string `db:"firstName"`
	LastName           *string `db:"lastName"`
	AvatarFilePath     *string `db:"avatarFilePath"`
	Status             string  `db:"status"`
}

type v2UserAuth struct {
	baseModel

	UserId   int    `db:"userId"`
	Source   string `db:"source"`
	SourceId string `db:"sourceId"`
}

type v2UserProjectLinkRel struct {
	baseModel

	UserId        int `db:"userId"`
	ProjectLinkId int `db:"projectLinkId"`
}

type v2UserProjectRel struct {
	baseModel

	UserId    int  `db:"userId"`
	ProjectId int  `db:"projectId"`
	Pinned    *int `db:"pinned"`
}

type v2UserScreenCommentRel struct {
	baseModel

	UserId          int  `db:"userId"`
	ScreenCommentId int  `db:"screenCommentId"`
	IsRead          *int `db:"isRead"`
	IsProcessed     *int `db:"isProcessed"`
}

type v2UserSetting struct {
	baseModel

	UserId int     `db:"userId"`
	Type   string  `db:"type"`
	Name   string  `db:"name"`
	Value  *string `db:"value"`
}
