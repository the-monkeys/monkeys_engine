package server

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_file_service/pb"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_file_storage/internal/utils"
)

type FileService struct {
	path           string
	profilePicPath string
	pb.UnimplementedUploadBlogFileServer
}

func NewFileService(path, profilePic string) *FileService {
	return &FileService{
		path:           path,
		profilePicPath: profilePic,
	}
}

func (fs *FileService) UploadBlogFile(stream pb.UploadBlogFile_UploadBlogFileServer) error {
	var byteSlice []byte
	var blogId string
	var fileName string
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		byteSlice = append(byteSlice, chunk.Data...)
		blogId = chunk.BlogId
		fileName = chunk.FileName
	}
	logrus.Infof("Uploading a file for blog id: %v", blogId)

	fileName = utils.RemoveSpecialChar(fileName)
	dirPath, filePath := utils.ConstructPath(fs.path, blogId, fileName)

	// Check if directory exists, if not create it
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		logrus.Infof("the directory, %s doesn't exists", dirPath)

		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			logrus.Errorf("cannot create a directory for this blog id: %s", blogId)
			return err
		}
	}

	// Check if file exists, if not create it with sample data
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		logrus.Infof("the file, %s doesn't exists", filePath)

		err := ioutil.WriteFile(filePath, byteSlice, 0644)
		if err != nil {
			logrus.Errorf("cannot create a file for this blog id: %s", blogId)
			return err
		}
	}

	logrus.Infof("done uploading file: %s", filePath)
	return stream.SendAndClose(&pb.UploadBlogFileRes{
		Status:      http.StatusOK,
		NewFileName: fileName,
	})
}

// TODO: return error 404 if file doesn't exist
func (fs *FileService) GetBlogFile(req *pb.GetBlogFileReq, stream pb.UploadBlogFile_GetBlogFileServer) error {
	fileName := filepath.Join(fs.path, req.BlogId, req.FileName)
	logrus.Infof("there is a request to retrieve the file, %s", fileName)

	rawFileName := strings.ReplaceAll(fileName, "\n", "")
	fileBytes, err := ioutil.ReadFile(rawFileName)
	if err != nil {
		logrus.Errorf("cannot read the file: %s, error: %v", fileName, fileBytes)
		return err
	}

	if err := stream.Send(&pb.GetBlogFileRes{
		Data: fileBytes,
	}); err != nil {
		logrus.Errorf("error while sending stream, error %+v", err)
	}

	return nil
}

func (fs *FileService) DeleteBlogFile(ctx context.Context, req *pb.DeleteBlogFileReq) (*pb.DeleteBlogFileRes, error) {
	filePath := filepath.Join(fs.path, req.BlogId, req.FileName)

	logrus.Infof("there is a request to delete the file, %s", filePath)

	if err := os.Remove(filePath); err != nil {
		return nil, err
	}

	return &pb.DeleteBlogFileRes{
		Message: "successfully deleted",
		Status:  http.StatusOK,
	}, nil
}

func (fs *FileService) UploadProfilePic(stream pb.UploadBlogFile_UploadProfilePicServer) error {
	logrus.Infof("File server got request to save profile pic")
	var byteSlice []byte
	var blogId string
	var fileName string
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		byteSlice = append(byteSlice, chunk.Data...)
		blogId = chunk.UserId
		fileName = chunk.FileType
	}
	logrus.Infof("Uploading a file for user id: %v", blogId)

	fileName = "profile.png"
	dirPath, filePath := utils.ConstructPath(fs.profilePicPath, blogId, fileName)

	// Check if directory exists, if not create it
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		logrus.Infof("the directory, %s doesn't exists", dirPath)

		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			logrus.Errorf("cannot create a directory for this blog id: %s", blogId)
			return err
		}
	}

	err := ioutil.WriteFile(filePath, byteSlice, 0644)
	if err != nil {
		logrus.Errorf("cannot create a file for this blog id: %s", blogId)
		return err
	}

	logrus.Infof("done uploading profile pic: %s", filePath)
	return stream.SendAndClose(&pb.UploadProfilePicRes{
		Status:   http.StatusOK,
		FileName: fileName,
	})

}

func (fs *FileService) GetProfilePic(req *pb.GetProfilePicReq, stream pb.UploadBlogFile_GetProfilePicServer) error {
	logrus.Infof("File server got request to retrieve profile pic")
	fileName := filepath.Join(fs.profilePicPath, req.UserId, req.FileName)
	logrus.Infof("there is a request to retrieve the profile pic for user, %s", req.UserId)

	fileBytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		logrus.Errorf("cannot read the profile pic: %s, error: %v", fileName, fileBytes)
		return err
	}

	if err := stream.Send(&pb.GetProfilePicRes{
		Data: fileBytes,
	}); err != nil {
		logrus.Errorf("error while sending profile pic stream, error %+v", err)
	}

	return nil
}

func (fs *FileService) DeleteProfilePic(ctx context.Context, req *pb.DeleteProfilePicReq) (*pb.DeleteProfilePicRes, error) {
	logrus.Infof("File server got request to delete profile pic")
	filePath := filepath.Join(fs.profilePicPath, req.UserId, req.FileName)

	logrus.Infof("there is a request to delete the profile pic for user, %s", req.UserId)

	if err := os.Remove(filePath); err != nil {
		return nil, err
	}

	return &pb.DeleteProfilePicRes{
		Message: "successfully deleted",
		Status:  http.StatusOK,
	}, nil
}
