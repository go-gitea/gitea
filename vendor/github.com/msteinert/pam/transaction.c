#include "_cgo_export.h"
#include <security/pam_appl.h>
#include <string.h>

int cb_pam_conv(
	int num_msg,
	const struct pam_message **msg,
	struct pam_response **resp,
	void *appdata_ptr)
{
	*resp = calloc(num_msg, sizeof **resp);
	if (num_msg <= 0 || num_msg > PAM_MAX_NUM_MSG) {
		return PAM_CONV_ERR;
	}
	if (!*resp) {
		return PAM_BUF_ERR;
	}
	for (size_t i = 0; i < num_msg; ++i) {
		struct cbPAMConv_return result = cbPAMConv(
				msg[i]->msg_style,
				(char *)msg[i]->msg,
				(long)appdata_ptr);
		if (result.r1 != PAM_SUCCESS) {
			goto error;
		}
		(*resp)[i].resp = result.r0;
	}
	return PAM_SUCCESS;
error:
	for (size_t i = 0; i < num_msg; ++i) {
		if ((*resp)[i].resp) {
			memset((*resp)[i].resp, 0, strlen((*resp)[i].resp));
			free((*resp)[i].resp);
		}
	}
	memset(*resp, 0, num_msg * sizeof *resp);
	free(*resp);
	*resp = NULL;
	return PAM_CONV_ERR;
}

void init_pam_conv(struct pam_conv *conv, long c)
{
	conv->conv = cb_pam_conv;
	conv->appdata_ptr = (void *)c;
}
