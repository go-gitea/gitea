package setting

// FileStorage represents where to save avatars
var FileStorage struct {
	Bucket       string
	BucketURL    string
	SaveToBucket bool
}

func newFileStorage() {
	sec := Cfg.Section("storage")
	FileStorage.SaveToBucket = sec.HasKey("BUCKET")

	if FileStorage.SaveToBucket {
		FileStorage.Bucket = sec.Key("BUCKET").String()        // Preferred: "gs://<bucket-name>"
		FileStorage.BucketURL = sec.Key("BUCKET_URL").String() // Preferred: "https://storage.googleapis.com/<bucket-name>"
	}
	// Default Credential path for GoogleStorage => $HOME/.config/gcloud/application_default_credentials.json
}
