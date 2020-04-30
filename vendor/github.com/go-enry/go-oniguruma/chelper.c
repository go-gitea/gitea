#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#ifdef BENCHMARK_CHELP
#include <sys/time.h>
#endif
#include "chelper.h"

int NewOnigRegex( char *pattern, int pattern_length, int option,
                  OnigRegex *regex, OnigRegion **region, OnigEncoding *encoding, OnigErrorInfo **error_info, char **error_buffer) {
    int ret = ONIG_NORMAL;
    int error_msg_len = 0;

    OnigUChar *pattern_start = (OnigUChar *) pattern;
    OnigUChar *pattern_end = (OnigUChar *) (pattern + pattern_length);

    *error_info = (OnigErrorInfo *) malloc(sizeof(OnigErrorInfo));
    memset(*error_info, 0, sizeof(OnigErrorInfo));

    onig_initialize_encoding(*encoding);

    *error_buffer = (char*) malloc(ONIG_MAX_ERROR_MESSAGE_LEN * sizeof(char));

    memset(*error_buffer, 0, ONIG_MAX_ERROR_MESSAGE_LEN * sizeof(char));

    *region = onig_region_new();

    ret = onig_new(regex, pattern_start, pattern_end, (OnigOptionType)(option), *encoding, OnigDefaultSyntax, *error_info);

    if (ret != ONIG_NORMAL) {
        error_msg_len = onig_error_code_to_str((unsigned char*)(*error_buffer), ret, *error_info);
        if (error_msg_len >= ONIG_MAX_ERROR_MESSAGE_LEN) {
            error_msg_len = ONIG_MAX_ERROR_MESSAGE_LEN - 1;
        }
        (*error_buffer)[error_msg_len] = '\0';
    }
    return ret;
}

int SearchOnigRegex( void *str, int str_length, int offset, int option,
                  OnigRegex regex, OnigRegion *region, OnigErrorInfo *error_info, char *error_buffer, int *captures, int *numCaptures) {
    int ret = ONIG_MISMATCH;
    int error_msg_len = 0;
#ifdef BENCHMARK_CHELP
    struct timeval tim1, tim2;
    long t;
#endif

    OnigUChar *str_start = (OnigUChar *) str;
    OnigUChar *str_end = (OnigUChar *) (str_start + str_length);
    OnigUChar *search_start = (OnigUChar *)(str_start + offset);
    OnigUChar *search_end = str_end;

#ifdef BENCHMARK_CHELP
    gettimeofday(&tim1, NULL);
#endif

    ret = onig_search(regex, str_start, str_end, search_start, search_end, region, option);
    if (ret < 0 && error_buffer != NULL) {
        error_msg_len = onig_error_code_to_str((unsigned char*)(error_buffer), ret, error_info);
        if (error_msg_len >= ONIG_MAX_ERROR_MESSAGE_LEN) {
            error_msg_len = ONIG_MAX_ERROR_MESSAGE_LEN - 1;
        }
        error_buffer[error_msg_len] = '\0';
    }
    else if (captures != NULL) {
        int i;
        int count = 0;
        for (i = 0; i < region->num_regs; i++) {
            captures[2*count] = region->beg[i];
            captures[2*count+1] = region->end[i];
            count ++;
        }
        *numCaptures = count;
    }

#ifdef BENCHMARK_CHELP
    gettimeofday(&tim2, NULL);
    t = (tim2.tv_sec - tim1.tv_sec) * 1000000 + tim2.tv_usec - tim1.tv_usec;
    printf("%ld microseconds elapsed\n", t);
#endif
    return ret;
}

int MatchOnigRegex(void *str, int str_length, int offset, int option,
                  OnigRegex regex, OnigRegion *region) {
    int ret = ONIG_MISMATCH;
    int error_msg_len = 0;
#ifdef BENCHMARK_CHELP
    struct timeval tim1, tim2;
    long t;
#endif

    OnigUChar *str_start = (OnigUChar *) str;
    OnigUChar *str_end = (OnigUChar *) (str_start + str_length);
    OnigUChar *search_start = (OnigUChar *)(str_start + offset);

#ifdef BENCHMARK_CHELP
    gettimeofday(&tim1, NULL);
#endif
    ret = onig_match(regex, str_start, str_end, search_start, region, option);
#ifdef BENCHMARK_CHELP
    gettimeofday(&tim2, NULL);
    t = (tim2.tv_sec - tim1.tv_sec) * 1000000 + tim2.tv_usec - tim1.tv_usec;
    printf("%ld microseconds elapsed\n", t);
#endif
    return ret;
}

int LookupOnigCaptureByName(char *name, int name_length,
                  OnigRegex regex, OnigRegion *region) {
    int ret = ONIGERR_UNDEFINED_NAME_REFERENCE;
#ifdef BENCHMARK_CHELP
    struct timeval tim1, tim2;
    long t;
#endif
    OnigUChar *name_start = (OnigUChar *) name;
    OnigUChar *name_end = (OnigUChar *) (name_start + name_length);
#ifdef BENCHMARK_CHELP
    gettimeofday(&tim1, NULL);
#endif
    ret = onig_name_to_backref_number(regex, name_start, name_end, region);
#ifdef BENCHMARK_CHELP
    gettimeofday(&tim2, NULL);
    t = (tim2.tv_sec - tim1.tv_sec) * 1000000 + tim2.tv_usec - tim1.tv_usec;
    printf("%ld microseconds elapsed\n", t);
#endif
    return ret;
}

typedef struct {
    char *nameBuffer;
    int bufferOffset;
    int bufferSize;
    int *numbers;
    int numIndex;
} group_info_t;

int name_callback(const UChar* name, const UChar* name_end,
          int ngroup_num, int* group_nums,
          regex_t* reg, void* arg)
{
    int nameLen, offset, newOffset;
    group_info_t *groupInfo;

    groupInfo = (group_info_t*) arg;
    offset = groupInfo->bufferOffset;
    nameLen = name_end - name;
    newOffset = offset + nameLen;

    //if there are already names, add a ";"
    if (offset > 0) {
        newOffset += 1;
    }

    if (newOffset <= groupInfo->bufferSize) {
        if (offset > 0) {
            groupInfo->nameBuffer[offset] = ';';
            offset += 1;
        }
        memcpy(&groupInfo->nameBuffer[offset], name, nameLen);
    }
    groupInfo->bufferOffset = newOffset;
    if (ngroup_num > 0) {
        groupInfo->numbers[groupInfo->numIndex] = group_nums[ngroup_num-1];
    } else {
        groupInfo->numbers[groupInfo->numIndex] = -1;
    }
    groupInfo->numIndex += 1;
    return 0;  /* 0: continue */
}

int GetCaptureNames(OnigRegex reg, void *buffer, int bufferSize, int* groupNumbers) {
    int ret;
    group_info_t groupInfo;
    groupInfo.nameBuffer = (char*)buffer;
    groupInfo.bufferOffset = 0;
    groupInfo.bufferSize = bufferSize;
    groupInfo.numbers = groupNumbers;
    groupInfo.numIndex = 0;
    onig_foreach_name(reg, name_callback, (void* )&groupInfo);
    return groupInfo.bufferOffset;
}

