// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	mail "code.gitea.io/gitea/services/mailer"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestEmailEmbedBase64Images(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	tests.PrepareAttachmentsStorage(t)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, Owner: user})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2, Repo: repo, Poster: user})

	attachment := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 13, IssueID: issue.ID, RepoID: repo.ID})
	ctx0 := context.Background()

	ctx := &mail.MailCommentContext{Context: ctx0 /* TODO: use a correct context */, Issue: issue, Doer: user}

	img1ExternalURL := "https://via.placeholder.com/10"
	img1ExternalImg := "<img src=\"" + img1ExternalURL + "\"/>"

	img2InternalURL := setting.AppURL + repo.Owner.Name + "/" + repo.Name + "/attachments/" + attachment.UUID
	img2InternalImg := "<img src=\"" + img2InternalURL + "\"/>"
	img2InternalBase64 := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAFAAAAAxCAYAAABNuS5SAAAAAXNSR0IArs4c6QAABWxJREFUaEPVmm1sU1UYx/9n3RsuxmS+ECdC71YYbCjohmMDNxyJONBNxaIo29qNlZqYbTFhi0FkmZpgogH0w1g7t46ZmEA0KIl+MQsvwQSM8QOJSGDtFO0XFTQE0tG11/Rqa9fel3Pf7+6H7cN9zjnP8zv/53nOvbcElFe7n/mOsKimNAfAfpWTm3NgtCP4Df0Ya1q2DpXdl2OL7yYEbrC4O+FlwBMiif/cH6Grc4RZEotj2lphsddAyB9gSbSlqvvvZx/tqQCQ5x4pBVgUGelrAqIgQJePYY10Rslam1d7w841fSWJsW5/qZIpVI0hBM5sgCyIy8/EVc1s0OCmh3eFt9X0mwYwK4V3+h2LZtnYVYPiV73MUw91hV9c+4Z1AM6HtE2nvmllR/il2jetAZAGHgu8M+4J7U0PgmacaqkJTPDkSvdv22v3PmBWDUylsMtnPwiQHqFAky2b776ZADdWtv26o25gkQUACndcMXgJx80E+MSKV35pW//2YssClIJnNsCG5dt/dj3+7hJTAbp8zAyAfL70tDrA+vJt0+76/XazAfIemGngma3A9eXOYGf9e9wJ2oyDNNdEhGoYPUB7P0D269VpxeatW/r8VNeG98vmNUAzVVhb1nLF03jAYRZAkhurVK1AowDeteBeRGZvYSZ6MyXKmtJnLns3HlpqFkDuZYLaFDYC4FhXMCuTkzUvec+MGjgvAPLBS9IcmuzB+akTZpRfbk3tAPqZU2BRr0ckYgDTVWg5BSYJ00LR44lEDN7E2X2Y/HECvo6LyLMVpNw0EOTtgCdUQDzDJXfcJgX/V+Y0YrRHGb3qIK36+DZZb5BzXukLqYeNo2HcGzpNo0KtFSgELx3MBy+fRXHR/aLu6QWSCqCZacwPkIXbz52bU5eYStPtOkcciLPavWjPBHgSQIPQVtKmslYqFIJy5tIxjJ7un+NmY0UbWtcN0CQJZ6OVIrO+ykkFTwNRag7aKGlqX+ZcK0rq0LflE6ol1EJMZ5H6qNQ+zNwiBAvEPDACohi8q39ewlufN6F13SAaK3ZwroZ+v4DB4y2yUloXgPSdlBwKeIK9QqDVqpBGfXw2gTN7cOqnTzm3pOqibgDpIf6Lj0+RagCKBX4jcg3dE9XYvMoL52N9vPt3I3Id3RNVogC1hJdwIuu7sPMobEV/MbNUxWSu0UzAEyqUuwm0HTXz2VfIv4Rd76ZRrFq8gddEd4CJVZs+dBQsLIxFFEBUPIQm7WrKmuFtPCi4hhRktfD4sk70tzFq0lEuSaW1L30dvQHylSxRgGrSUQ5AmqcOx8Iq7Gk+Jjjtzo+XIRafFax/eqiPtwbq1V3FgNIAlOqueqtPqGlKKpAvcK1TWy3Aw5O9ODf1JQa3fo0Hi8uzXNZLfbIUKKzM0jUAe15OymbaqgVolvo0ASgGTo5ShSDG2RhyiI3qSCL26l/NBos9gSlKYVpn5ABMzil1nOHrumJj9Uxf3RWopovTgMyEkzlGC3gkl1071jF9Tkg0uipQDUAaRb52pAo3Z65zpmakryEK1AKi1BGGTx1aqO/OiK3wo+4rid8OCV66K5ADOGZfjSj5gbZ2itkNuS6gME/6x/haAKR5fWcIQK1UmA5233MnYL+nkqo7K9k4GniGpXAyACVdWSr4rdW78fQjr6bMvr18HP6Tr0sNk7xvSYB6KFGShAIDWniGKzCxYLuPaSbAFwriMmSIHHimAPxPhTEAOYYQkbGIPRyyDQxA1rdPw5pIZhwuH2MpiCzIC+Oe4GcyeHOmpgHklOhnjoKFU67TWtvns8X5vl3fR5XMaypAPbszLQy5NS9zXksA5NQ4vGw5SPQibeBq7dSCS67/D+Q9UQwzW88cAAAAAElFTkSuQmCC"
	img2InternalBase64Img := "<img src=\"" + img2InternalBase64 + "\"/>"

	// 1st Test: convert internal image to base64
	t.Run("replaceSpecifiedBase64ImagesInternal", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		resultImg1Internal, err := mail.AttachmentSrcToBase64DataURI(img2InternalURL, ctx)
		assert.NoError(t, err)
		assert.Equal(t, img2InternalBase64, resultImg1Internal) // replace cause internal image
	})

	// 2nd Test: convert external image to base64 -> abort cause external image
	t.Run("replaceSpecifiedBase64ImagesExternal", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		resultImg1External, err := mail.AttachmentSrcToBase64DataURI(img1ExternalURL, ctx)
		assert.Error(t, err)
		assert.Equal(t, "", resultImg1External) // don't replace cause external image
	})

	// 3rd Test: generate email body with 1 internal and 1 external image, expect the result to have the internal image replaced with base64 data and the external not replaced
	t.Run("generateEmailBody", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		mailBody := "<html><head></head><body><p>Test1</p>" + img1ExternalImg + "<p>Test2</p>" + img2InternalImg + "<p>Test3</p></body></html>"
		expectedMailBody := "<html><head></head><body><p>Test1</p>" + img1ExternalImg + "<p>Test2</p>" + img2InternalBase64Img + "<p>Test3</p></body></html>"
		resultMailBody, err := mail.Base64InlineImages(mailBody, ctx)

		assert.NoError(t, err)
		assert.Equal(t, expectedMailBody, resultMailBody)
	})
}
