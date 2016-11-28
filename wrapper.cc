#include "cld2/public/compact_lang_det.h"
#include "cld2/public/encodings.h"
#include "wrapper.h"
#include <string.h>

extern "C" {
    const char* detect_language(const char *text) {
        int length = strlen(text);
        bool isPlainText = true;
        bool isReliable = true;
        CLD2::Language lang;

        lang = CLD2::DetectLanguage(text, length, isPlainText, &isReliable);

        return CLD2::LanguageCode(lang);
    }
}
