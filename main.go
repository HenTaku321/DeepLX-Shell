package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
)

type DeepLXResp struct {
	Code         int      `json:"code"`
	ID           int      `json:"ID"`
	Data         string   `json:"data"`
	Alternatives []string `json:"alternatives"`
}

type DeepLXReq struct {
	Text       string `json:"text"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
}

func setEnv() {
	path := os.Expand("${HOME}/.config/deeplx-for-command-line/", os.Getenv)
	cfg, err := os.OpenFile(path+"config.cfg", os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		if errors.As(err, &os.ErrNotExist) {
			err = os.MkdirAll(path, 0755)
			if err != nil {
				panic(err)
			}
			fmt.Println("成功创建配置文件目录, 请重新运行此程序: ", path)
			os.Exit(0)
		}
		panic(err)
	}
	defer cfg.Close()

	scanner := bufio.NewScanner(cfg)

	cfgIsEmpty := true
	for scanner.Scan() {
		cfgIsEmpty = false
		env := strings.Split(scanner.Text(), "=")

		if len(env) > 1 {
			err := os.Setenv(env[0], env[1])
			if err != nil {
				panic(err)
			}
		}
	}

	if cfgIsEmpty {
		fmt.Println("配置文件为空, 输出-h查看格式后填写在", cfg.Name())
		os.Exit(1)
	}

	if scanner.Err() != nil {
		panic(scanner.Err())
	}
}

func parseDeepLX(sourceText, sourceLang, targetLang string) ([]byte, error) {
	reqBody, err := json.Marshal(DeepLXReq{
		Text:       sourceText,
		SourceLang: sourceLang,
		TargetLang: targetLang,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", os.Getenv("API"), bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func transToEN(sourceText string) error {
	dlxResp := DeepLXResp{}

	text, err := parseDeepLX(sourceText, "ZH", "EN")
	if err != nil {
		return err
	}

	err = json.Unmarshal(text, &dlxResp)
	if err != nil {
		return err
	}

	fmt.Println(dlxResp.Data)
	return nil
}

func transToZH(sourceText string) error {
	dlxResp := DeepLXResp{}

	text, err := parseDeepLX(sourceText, "EN", "ZH")
	if err != nil {
		return err
	}

	err = json.Unmarshal(text, &dlxResp)
	if err != nil {
		return err
	}

	fmt.Println(dlxResp.Data)
	return nil
}

var (
	langCode               = regexp.MustCompile("[A-Z]{2}:[A-Z]{2}(?:-(?:[A-Z]{2}|[A-Z]{4}))?$")
	targetLangNotSupported = regexp.MustCompile(`Value for 'target_lang' not supported`)
)

func transToCustomized(sourceText, lang string) error {
	var (
		dlxResp DeepLXResp
		text    []byte
		err     error
	)

	if lang == "" || !langCode.MatchString(lang) {
		text, err = parseDeepLX(sourceText, os.Getenv("SourceLang"), os.Getenv("TargetLang"))
		if err != nil {
			return err
		}
	} else {
		customizedSourceLang, customizedTargetLang := strings.Split(lang, ":")[0], strings.Split(lang, ":")[1]
		text, err = parseDeepLX(sourceText, customizedSourceLang, customizedTargetLang)
		if err != nil {
			return err
		}
	}

	if targetLangNotSupported.Match(text) {
		return errors.New("错误的语言代码, 你可以去看https://developers.deepl.com/docs/api-reference/translate/openapi-spec-for-text-translation")
	}

	err = json.Unmarshal(text, &dlxResp)

	if err != nil {
		return err
	}

	fmt.Println(dlxResp.Data)
	return nil
}

func transFile(filePath, lang string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	sourceText, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	var (
		dlxResp DeepLXResp
		text    []byte
	)

	if lang == "" || !langCode.MatchString(lang) {
		text, err = parseDeepLX(string(sourceText), os.Getenv("SourceLang"), os.Getenv("TargetLang"))
		if err != nil {
			return err
		}
	} else {
		customizedSourceLang, customizedTargetLang := strings.Split(lang, ":")[0], strings.Split(lang, ":")[1]
		text, err = parseDeepLX(string(sourceText), customizedSourceLang, customizedTargetLang)
		if err != nil {
			return err
		}
	}

	if targetLangNotSupported.Match(text) {
		return errors.New("错误的语言代码, 你可以去看https://developers.deepl.com/docs/api-reference/translate/openapi-spec-for-text-translation")
	}

	err = json.Unmarshal(text, &dlxResp)
	if err != nil {
		return err
	}

	fmt.Println(dlxResp.Data)
	return nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法: trans [选项]")
		fmt.Fprintln(os.Stderr, "-z 将文本翻译为简体中文")
		fmt.Fprintln(os.Stderr, "-e 将文本翻译成英文")
		fmt.Fprintln(os.Stderr, "-l 自定义原和目标语言代码(示例: -l EN:ZH, 注意语言代码要大写)")
		fmt.Fprintln(os.Stderr, "-c 将文本翻译为自定义语言(示例: '-l EN:ZH -c Hello' 或 -c Hello)")
		fmt.Fprintln(os.Stderr, "-f 指定文件翻译, 同样支持-l")
		fmt.Fprintln(os.Stderr, "-h 显示帮助信息\n")
		fmt.Fprintln(os.Stderr, "无主参数进入循环, 使用-l或cfg中的语言")
		fmt.Fprintln(os.Stderr, "配置文件格式: \n一行一个, 必填内容:")
		fmt.Fprintln(os.Stderr, "API=DeepLX的URL")
		fmt.Fprintln(os.Stderr, "选填内容:")
		fmt.Fprintln(os.Stderr, "SourceLang=自定义你想要翻译的原语言(如EN, ZH, JA)")
		fmt.Fprintln(os.Stderr, "TargetLang=自定义你想要翻译的目标语言(如EN, ZH, JA)")
	}
	targetLangIsZH := flag.String("z", "", "将文本翻译为简体中文")
	targetLangIsEN := flag.String("e", "", "将文本翻译为英文")
	lang := flag.String("l", "", "指定原和目标语言代码(示例: -l EN:ZH), 如果不填则使用cfg中的自定义语言(示例: -c Hello)")
	targetLangIsCustomized := flag.String("c", "", "将文本翻译为自定义语言(示例: '-l EN:ZH -c Hello' 或 -c Hello)")
	targetLangIsFile := flag.String("f", "", "指定文件翻译, 同样支持-l")

	flag.Parse()

	setEnv()

	if *targetLangIsEN != "" {
		err := transToEN(*targetLangIsEN)
		if err != nil {
			panic(err)
		}
	} else if *targetLangIsZH != "" {
		err := transToZH(*targetLangIsZH)
		if err != nil {
			panic(err)
		}
	} else if *targetLangIsCustomized != "" {
		err := transToCustomized(*targetLangIsCustomized, *lang)
		if err != nil {
			panic(err)
		}
	} else if *targetLangIsFile != "" {
		err := transFile(*targetLangIsFile, *lang)
		if err != nil {
			panic(err)
		}
	} else { //其他标志
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			err := transToCustomized(scanner.Text(), *lang)
			if err != nil {
				panic(err)
			}
		}

		if scanner.Err() != nil {
			panic(scanner.Err())
		}
	}
}
