#include <oniguruma.h>

extern int NewOnigRegex( char *pattern, int pattern_length, int option,
                                  OnigRegex *regex, OnigEncoding *encoding, OnigErrorInfo **error_info, char **error_buffer);

extern int SearchOnigRegex( void *str, int str_length, int offset, int option,
                                  OnigRegex regex, OnigErrorInfo *error_info, char *error_buffer, int *captures, int *numCaptures);

extern int MatchOnigRegex( void *str, int str_length, int offset, int option,
                  OnigRegex regex);

extern int LookupOnigCaptureByName(char *name, int name_length, OnigRegex regex);

extern int GetCaptureNames(OnigRegex regex, void *buffer, int bufferSize, int* groupNumbers);
