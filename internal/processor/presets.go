package processor

// SocialPreset defines a social media image preset.
type SocialPreset struct {
	Name        string `json:"name"`
	Platform    string `json:"platform"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Format      string `json:"format"`
	MaxFileSize int    `json:"maxFileSize"`
	Description string `json:"description"`
}

// SocialPresets is the master list of all social media presets.
var SocialPresets = []SocialPreset{
	// Existing presets (from the current HTML select)
	{"Instagram Post", "instagram", 1080, 1080, "jpeg", 0, "Square format"},
	{"Instagram Portrait", "instagram", 1080, 1350, "jpeg", 0, "Portrait format"},
	{"Instagram Story", "instagram", 1080, 1920, "jpeg", 0, "Story/Reels format"},
	{"Facebook Post", "facebook", 1200, 630, "jpeg", 0, "Link preview format"},
	{"Twitter Post", "twitter", 1200, 675, "jpeg", 0, "In-feed image"},
	{"YouTube Thumbnail", "youtube", 1280, 720, "jpeg", 0, "Video thumbnail"},
	{"LinkedIn Post", "linkedin", 1200, 627, "jpeg", 0, "In-feed image"},

	// New presets
	{"LinkedIn Banner", "linkedin", 1584, 396, "jpeg", 0, "Profile banner"},
	{"Twitter/X Header", "twitter", 1500, 500, "jpeg", 0, "Profile header"},
	{"Slack Emoji", "slack", 128, 128, "png", 256, "128x128, max 256KB"},
	{"Discord Emoji", "discord", 128, 128, "png", 256, "128x128, max 256KB"},
	{"Pinterest Pin", "pinterest", 1000, 1500, "jpeg", 0, "Standard pin"},
	{"Pinterest Long Pin", "pinterest", 1000, 2100, "jpeg", 0, "Long-form pin"},
	{"Favicon Pack", "favicon", 512, 512, "png", 0, "Generate all favicon sizes"},
	{"Twitch Panel", "twitch", 320, 160, "png", 0, "About me panel"},
	{"Twitch Banner", "twitch", 1200, 480, "jpeg", 0, "Channel banner"},
	{"App Store iOS", "appstore", 1242, 2688, "jpeg", 0, "iPhone Xs Max"},
	{"App Store Android", "appstore", 1080, 1920, "jpeg", 0, "Standard Android"},
	{"Instagram Carousel", "instagram", 1080, 1350, "jpeg", 0, "Carousel slide"},
}

// GetSocialPreset returns a preset by name.
func GetSocialPreset(name string) *SocialPreset {
	for i := range SocialPresets {
		if SocialPresets[i].Name == name {
			return &SocialPresets[i]
		}
	}
	return nil
}
