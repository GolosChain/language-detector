import json
import sys

if __name__ == "__main__":

    languages = {}

    for line in open("cld_codes.txt", "r"):
        info = line.split(',')

        if len(info) > 2:
            language = info[0].strip().strip('{"')
            code = info[1].strip().strip('"')

            if code in languages:
                languages[code].append(language)
            else:
                languages[code] = [ language ]

    print "Total languages: ", len(languages)
    print "Overlaps: "
    overlapped = False
    for code in languages:
        names = languages[code]
        if len(names) > 1:
            print code, ": ", names
            overlapped = True
        else:
            languages[code] = names[0].capitalize()

    if not overlapped:
        outfile = open("cld_codes.json", "w")
        outfile.write(json.dumps(languages, indent=4, separators=(',', ': '), sort_keys=True))
        outfile.close()
        print "None! Result saved in cld_codes.json"
