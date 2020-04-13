package codescan

import "regexp"

const (
	rxMethod = "(\\p{L}+)"
	rxPath   = "((?:/[\\p{L}\\p{N}\\p{Pd}\\p{Pc}{}\\-\\.\\?_~%!$&'()*+,;=:@/]*)+/?)"
	rxOpTags = "(\\p{L}[\\p{L}\\p{N}\\p{Pd}\\.\\p{Pc}\\p{Zs}]+)"
	rxOpID   = "((?:\\p{L}[\\p{L}\\p{N}\\p{Pd}\\p{Pc}]+)+)"

	rxMaximumFmt    = "%s[Mm]ax(?:imum)?\\p{Zs}*:\\p{Zs}*([\\<=])?\\p{Zs}*([\\+-]?(?:\\p{N}+\\.)?\\p{N}+)$"
	rxMinimumFmt    = "%s[Mm]in(?:imum)?\\p{Zs}*:\\p{Zs}*([\\>=])?\\p{Zs}*([\\+-]?(?:\\p{N}+\\.)?\\p{N}+)$"
	rxMultipleOfFmt = "%s[Mm]ultiple\\p{Zs}*[Oo]f\\p{Zs}*:\\p{Zs}*([\\+-]?(?:\\p{N}+\\.)?\\p{N}+)$"

	rxMaxLengthFmt        = "%s[Mm]ax(?:imum)?(?:\\p{Zs}*[\\p{Pd}\\p{Pc}]?[Ll]en(?:gth)?)\\p{Zs}*:\\p{Zs}*(\\p{N}+)$"
	rxMinLengthFmt        = "%s[Mm]in(?:imum)?(?:\\p{Zs}*[\\p{Pd}\\p{Pc}]?[Ll]en(?:gth)?)\\p{Zs}*:\\p{Zs}*(\\p{N}+)$"
	rxPatternFmt          = "%s[Pp]attern\\p{Zs}*:\\p{Zs}*(.*)$"
	rxCollectionFormatFmt = "%s[Cc]ollection(?:\\p{Zs}*[\\p{Pd}\\p{Pc}]?[Ff]ormat)\\p{Zs}*:\\p{Zs}*(.*)$"
	rxEnumFmt             = "%s[Ee]num\\p{Zs}*:\\p{Zs}*(.*)$"
	rxDefaultFmt          = "%s[Dd]efault\\p{Zs}*:\\p{Zs}*(.*)$"
	rxExampleFmt          = "%s[Ee]xample\\p{Zs}*:\\p{Zs}*(.*)$"

	rxMaxItemsFmt = "%s[Mm]ax(?:imum)?(?:\\p{Zs}*|[\\p{Pd}\\p{Pc}]|\\.)?[Ii]tems\\p{Zs}*:\\p{Zs}*(\\p{N}+)$"
	rxMinItemsFmt = "%s[Mm]in(?:imum)?(?:\\p{Zs}*|[\\p{Pd}\\p{Pc}]|\\.)?[Ii]tems\\p{Zs}*:\\p{Zs}*(\\p{N}+)$"
	rxUniqueFmt   = "%s[Uu]nique\\p{Zs}*:\\p{Zs}*(true|false)$"

	rxItemsPrefixFmt = "(?:[Ii]tems[\\.\\p{Zs}]*){%d}"
)

var (
	rxSwaggerAnnotation  = regexp.MustCompile(`swagger:([\p{L}\p{N}\p{Pd}\p{Pc}]+)`)
	rxFileUpload         = regexp.MustCompile(`swagger:file`)
	rxStrFmt             = regexp.MustCompile(`swagger:strfmt\p{Zs}*(\p{L}[\p{L}\p{N}\p{Pd}\p{Pc}]+)$`)
	rxAlias              = regexp.MustCompile(`swagger:alias`)
	rxName               = regexp.MustCompile(`swagger:name\p{Zs}*(\p{L}[\p{L}\p{N}\p{Pd}\p{Pc}\.]+)$`)
	rxAllOf              = regexp.MustCompile(`swagger:allOf\p{Zs}*(\p{L}[\p{L}\p{N}\p{Pd}\p{Pc}\.]+)?$`)
	rxModelOverride      = regexp.MustCompile(`swagger:model\p{Zs}*(\p{L}[\p{L}\p{N}\p{Pd}\p{Pc}]+)?$`)
	rxResponseOverride   = regexp.MustCompile(`swagger:response\p{Zs}*(\p{L}[\p{L}\p{N}\p{Pd}\p{Pc}]+)?$`)
	rxParametersOverride = regexp.MustCompile(`swagger:parameters\p{Zs}*(\p{L}[\p{L}\p{N}\p{Pd}\p{Pc}\p{Zs}]+)$`)
	rxEnum               = regexp.MustCompile(`swagger:enum\p{Zs}*(\p{L}[\p{L}\p{N}\p{Pd}\p{Pc}]+)$`)
	rxIgnoreOverride     = regexp.MustCompile(`swagger:ignore\p{Zs}*(\p{L}[\p{L}\p{N}\p{Pd}\p{Pc}]+)?$`)
	rxDefault            = regexp.MustCompile(`swagger:default\p{Zs}*(\p{L}[\p{L}\p{N}\p{Pd}\p{Pc}]+)$`)
	rxType               = regexp.MustCompile(`swagger:type\p{Zs}*(\p{L}[\p{L}\p{N}\p{Pd}\p{Pc}]+)$`)
	rxRoute              = regexp.MustCompile(
		"swagger:route\\p{Zs}*" +
			rxMethod +
			"\\p{Zs}*" +
			rxPath +
			"(?:\\p{Zs}+" +
			rxOpTags +
			")?\\p{Zs}+" +
			rxOpID + "\\p{Zs}*$")
	rxBeginYAMLSpec    = regexp.MustCompile(`---\p{Zs}*$`)
	rxUncommentHeaders = regexp.MustCompile(`^[\p{Zs}\t/\*-]*\|?`)
	rxUncommentYAML    = regexp.MustCompile(`^[\p{Zs}\t]*/*`)
	rxOperation        = regexp.MustCompile(
		"swagger:operation\\p{Zs}*" +
			rxMethod +
			"\\p{Zs}*" +
			rxPath +
			"(?:\\p{Zs}+" +
			rxOpTags +
			")?\\p{Zs}+" +
			rxOpID + "\\p{Zs}*$")

	rxSpace              = regexp.MustCompile(`\p{Zs}+`)
	rxIndent             = regexp.MustCompile(`\p{Zs}*/*\p{Zs}*[^\p{Zs}]`)
	rxPunctuationEnd     = regexp.MustCompile(`\p{Po}$`)
	rxStripComments      = regexp.MustCompile(`^[^\p{L}\p{N}\p{Pd}\p{Pc}\+]*`)
	rxStripTitleComments = regexp.MustCompile(`^[^\p{L}]*[Pp]ackage\p{Zs}+[^\p{Zs}]+\p{Zs}*`)
	rxAllowedExtensions  = regexp.MustCompile(`^[Xx]-`)

	rxIn              = regexp.MustCompile(`[Ii]n\p{Zs}*:\p{Zs}*(query|path|header|body|formData)$`)
	rxRequired        = regexp.MustCompile(`[Rr]equired\p{Zs}*:\p{Zs}*(true|false)$`)
	rxDiscriminator   = regexp.MustCompile(`[Dd]iscriminator\p{Zs}*:\p{Zs}*(true|false)$`)
	rxReadOnly        = regexp.MustCompile(`[Rr]ead(?:\p{Zs}*|[\p{Pd}\p{Pc}])?[Oo]nly\p{Zs}*:\p{Zs}*(true|false)$`)
	rxConsumes        = regexp.MustCompile(`[Cc]onsumes\p{Zs}*:`)
	rxProduces        = regexp.MustCompile(`[Pp]roduces\p{Zs}*:`)
	rxSecuritySchemes = regexp.MustCompile(`[Ss]ecurity\p{Zs}*:`)
	rxSecurity        = regexp.MustCompile(`[Ss]ecurity\p{Zs}*[Dd]efinitions:`)
	rxResponses       = regexp.MustCompile(`[Rr]esponses\p{Zs}*:`)
	rxParameters      = regexp.MustCompile(`[Pp]arameters\p{Zs}*:`)
	rxSchemes         = regexp.MustCompile(`[Ss]chemes\p{Zs}*:\p{Zs}*((?:(?:https?|HTTPS?|wss?|WSS?)[\p{Zs},]*)+)$`)
	rxVersion         = regexp.MustCompile(`[Vv]ersion\p{Zs}*:\p{Zs}*(.+)$`)
	rxHost            = regexp.MustCompile(`[Hh]ost\p{Zs}*:\p{Zs}*(.+)$`)
	rxBasePath        = regexp.MustCompile(`[Bb]ase\p{Zs}*-*[Pp]ath\p{Zs}*:\p{Zs}*` + rxPath + "$")
	rxLicense         = regexp.MustCompile(`[Ll]icense\p{Zs}*:\p{Zs}*(.+)$`)
	rxContact         = regexp.MustCompile(`[Cc]ontact\p{Zs}*-?(?:[Ii]info\p{Zs}*)?:\p{Zs}*(.+)$`)
	rxTOS             = regexp.MustCompile(`[Tt](:?erms)?\p{Zs}*-?[Oo]f?\p{Zs}*-?[Ss](?:ervice)?\p{Zs}*:`)
	rxExtensions      = regexp.MustCompile(`[Ee]xtensions\p{Zs}*:`)
	rxInfoExtensions  = regexp.MustCompile(`[In]nfo\p{Zs}*[Ee]xtensions:`)
	rxDeprecated      = regexp.MustCompile(`[Dd]eprecated\p{Zs}*:\p{Zs}*(true|false)$`)
	// currently unused: rxExample         = regexp.MustCompile(`[Ex]ample\p{Zs}*:\p{Zs}*(.*)$`)
)
