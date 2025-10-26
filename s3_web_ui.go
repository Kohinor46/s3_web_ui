package main

import (	
	"log"
	"io"
	"strings"
	"flag"
	"net/http"
	"net/url"
	"time"
	"os"
	"bytes"
	"math/rand"
	"html/template"
	"encoding/base64"
	"mime"
	"path"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/dustin/go-humanize"
)

type Config struct{
	S3 struct {
		User string
		Passw string
		Addr string
		Bucket string
	}
}

type Data struct{
	Mode string
	Name string
	Folders[]string
	Files []File

}

type File struct{
	Size uint64
	Name string
	Base64 string
	LastModified time.Time
}

var (
	mode string
	config Config
	Svc *s3.S3
)

func init() {
	flag.StringVar(&config.S3.User, "s3_user", "", "s3_user")
	flag.StringVar(&config.S3.Addr, "s3_addr", "", "s3_addr")
	flag.StringVar(&config.S3.Bucket, "s3_bucket", "", "s3_config_bucket")
	flag.StringVar(&config.S3.Passw, "s3_passw", "", "s3_passw")
	
	if config.S3.User==""{
		config.S3.User=os.Getenv("s3_user")
		if config.S3.User==""{
			log.Fatalf("s3_user Required")
		}
	}	
	if config.S3.Addr==""{
		config.S3.Addr=os.Getenv("s3_addr")
		if config.S3.Addr==""{
			log.Fatalf("s3_addr Required")
		}
	}	
	if config.S3.Bucket==""{
		config.S3.Bucket=os.Getenv("s3_bucket")
		if config.S3.Bucket==""{
			log.Fatalf("s3_bucket Required")
		}
	}	
	if config.S3.Passw==""{
		config.S3.Passw=os.Getenv("s3_passw")
		if config.S3.Passw==""{
			log.Fatalf("s3_passw Required")
		}
	}
}

func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost{
		const maxUpload = 2 << 30
		r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			log.Printf("parse form: %w", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		form := r.MultipartForm
		files := form.File["file"]
		if len(files) == 0 {
			log.Printf("no files in form field 'file'")
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		prefix := strings.TrimPrefix(r.URL.Path, "/")
		if prefix != "" && !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		for _, fh := range files {
			name:=strings.ReplaceAll(fh.Filename, "\\", "_")
			if name == "" {
				continue
			}
			f, err := fh.Open()
			if err != nil {
				log.Printf("open %q: %w", fh.Filename, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			body, ctype, err := withSniff(f, fh.Header.Get("Content-Type"))
			if err != nil {
				f.Close()
				log.Printf("sniff %q: %w", fh.Filename, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			name=prefix+name
			uploads3object(name,ctype,body)
		}
    	http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
    	return
	}


	if r.URL.RawQuery!=""{
		if strings.Contains(r.URL.RawQuery,"delete"){
			s:=strings.Split(r.URL.RawQuery,"!")
			name,err:=base64.StdEncoding.DecodeString(s[1])
			if err!=nil{
				log.Println("Error base64 DecodeString: ",err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
    			return
			}
			if err:=deletes3object(string(name),""); err!=nil{
				log.Println("Error delete ", name,":\n",err)
			}
			return
		}
		n,err:=url.QueryUnescape(r.URL.RawQuery)
		if err!=nil{
			log.Println("Error QueryUnescape:\n",err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
    		return
		}
		name,err:=base64.StdEncoding.DecodeString(n)
		if err!=nil{
			log.Println("Error base64 DecodeString: ",err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
    		return
		}
		body,err:=gets3object(string(name))
		if err!=nil{
			http.Error(w, err.Error(), http.StatusInternalServerError)
    		return
		}
		if _, err := io.Copy(w, body.Body); err != nil {
			// Клиент оборвал соединение — логируем как info
			log.Printf("stream aborted: %v", err)
		}
		return
	}
	funcs := template.FuncMap{
	    "i64": func(p *int64) int {
	        if p == nil { return 0 }
	        return int(*p)
	    },
	    "humanize": func(i uint64) string{
	    	return humanize.Bytes(i)
	    },
	    "time": func(t *time.Time) string{
	    	return t.Format("2006-01-02 15:04:05")
	    },
	    "add": func(x,y int) int{
	    	return x+y
	    },
	    "base64": func(s1,s2 string) string{
	    	return base64.StdEncoding.EncodeToString([]byte(s1+s2))
	    },
	}
	tmpl := template.Must(template.New("index.gohtml").Funcs(funcs).ParseFiles("index.gohtml"))
	data,err:=gets3objects(r.URL.String(),"")
	if err!=nil{
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	if r.URL.Path!="/"{
		data.Name=r.URL.Path
	}
	err = tmpl.Execute(w, data)
    if err != nil {
    	http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}

func withSniff(src io.Reader, hinted string) (io.Reader, string, error) {
	var head [512]byte
	n, err := io.ReadFull(src, head[:])
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, "", err
	}
	buf := head[:n]

	ctype := strings.TrimSpace(hinted)
	if ctype == "" || ctype == "application/octet-stream" {
		ctype = http.DetectContentType(buf)
	}
	// нормализуем по расширению, если нужно
	if ex := mime.TypeByExtension(path.Ext("x" /*placeholder*/)); ex != "" && ctype == "" {
		ctype = ex
	}
	return io.MultiReader(bytes.NewReader(buf), src), ctype, nil
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, "./s3.png")
}

func main(){
	creds := credentials.NewStaticCredentials(config.S3.User, config.S3.Passw, "")
	sess, err := session.NewSession(&aws.Config{
		Credentials:      creds,
		Region:           aws.String("ru-central1"),
		S3ForcePathStyle: aws.Bool(true),
		Endpoint:         aws.String(config.S3.Addr),
		LogLevel:         aws.LogLevel(aws.LogOff),
	})
	if err != nil {
		log.Fatalf("NewSession error: %s\n", err)
	}

	Svc = s3.New(sess)

	check_s3()

	http.HandleFunc("/favicon.ico", faviconHandler)
	http.HandleFunc("/", Handler)
	if err := http.ListenAndServe(":8080", nil); err!=nil{
		log.Fatalf("Server failed to start: %v\n", err)
	} 
}

func check_s3(){
	_, err := Svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:    aws.String(config.S3.Bucket),
		Delimiter: aws.String("/"),
	})
	if err==nil{
		mode="list"
		charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    	seed := rand.NewSource(time.Now().UnixNano())
    	random := rand.New(seed)
	
    	result := make([]byte, 10)
    	for i := range result {
    	    result[i] = charset[random.Intn(len(charset))]
    	}
		_, err := Svc.PutObject(&s3.PutObjectInput{
			Bucket:      aws.String(config.S3.Bucket),
			Key:         aws.String(string(result)),
			Body:        bytes.NewReader(result),
			ContentType: aws.String("text/plain"),
			//Metadata:    map[string]*string{"purpose": aws.String("permcheck")},
		})
		if err==nil{
			mode="put"
			_, err = Svc.DeleteObject(&s3.DeleteObjectInput{
				Bucket: aws.String(config.S3.Bucket),
				Key:    aws.String(string(result)),
			})
			if err==nil{
				mode="delete"
			}
		}
	}else{
		log.Fatalf("Can't list objects, grand permissions")
	}
}

func gets3object(name string) (s3.GetObjectOutput,error){
	getIn := &s3.GetObjectInput{
		Bucket: aws.String(config.S3.Bucket),
		Key:    aws.String(name),
	}
	out, err := Svc.GetObject(getIn)
	if err!=nil{
		log.Println("Error download: ",err)
		return s3.GetObjectOutput{},err
	}
	return *out, nil
}

func deletes3object(name,token string)error{
	if !strings.Contains(name,"/"){
		_, err := Svc.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(config.S3.Bucket),
			Key:    aws.String(name),
		})
		if err!=nil{
			return err
		}
		return nil
	}
	out, err := Svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:    aws.String(config.S3.Bucket),
		Prefix:    aws.String(name),
	})
	if err!=nil{
		log.Println("Error ListObjectsV2: ",err)
		return err
	}
	for _, content := range out.Contents {
		_, err = Svc.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(config.S3.Bucket),
			Key:    content.Key,
		})
		if err!=nil{
			return err
		}
	}
	return nil
}

func uploads3object(name,ctype string,body io.Reader) error {
	u := s3manager.NewUploaderWithClient(Svc)
	_, err := u.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(config.S3.Bucket),
		Key:         aws.String(name),
		Body:        body,
		ContentType: aws.String(ctype),
	})
	if err != nil {
		log.Printf("s3 upload: %w", err)
		return err
	}
	return nil
}

func gets3objects(prefix,token string) (Data,error){
	var d Data
	d.Mode=mode
	list := &s3.ListObjectsV2Input{
		Bucket:    aws.String(config.S3.Bucket),
		Delimiter: aws.String("/"),
		Prefix:    aws.String(prefix[1:]),
	}
	if token != "" {
		list.ContinuationToken = aws.String(token)
	}

	out, err := Svc.ListObjectsV2(list)
	if err!=nil{
		log.Println("Error ListObjectsV2: ",err)
		return d,err
	}
	
	for _, cp := range out.CommonPrefixes {
		c:=strings.Split(*cp.Prefix,"/")
		d.Folders=append(d.Folders,c[len(c)-2]+"/")
	}
	for _, content := range out.Contents {
		var f File
		f.Base64=base64.StdEncoding.EncodeToString([]byte(*content.Key))
		c:=strings.Split(*content.Key,"/")
		f.Name=c[len(c)-1]
		f.Size=uint64(*content.Size)
		f.LastModified=*content.LastModified
		d.Files=append(d.Files,f)
	}

	b:=*out.IsTruncated
	if b{
		data,err:=gets3objects(prefix,*out.NextContinuationToken)
		if err!=nil{
			return d, err
		}
		for _, folder := range data.Folders {
			d.Folders=append(d.Folders,folder)
		}
		for _, file := range data.Files {
			d.Files=append(d.Files,file)
		}
	}
	return d,nil
}
