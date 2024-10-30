package server

// type FileServer struct {
// 	profilePicPath string
// 	blogFilesPath  string
// 	log            *logrus.Logger
// 	pb.UnimplementedFileServiceServer
// }

// func NewFileServer(profilePicPath, blogFilesPath string, log *logrus.Logger) *FileServer {
// 	return &FileServer{
// 		profilePicPath: profilePicPath,
// 		blogFilesPath:  blogFilesPath,
// 		log:            log,
// 	}
// }

// func (fs *FileServer) UploadProfilePic(stream pb.FileService_UploadProfilePicServer) error {
// 	fs.log.Infof("File server got request to save profile pic")
// 	return nil
// }

// func (fs *FileServer) GetProfilePic(ctx context.Context, req *pb.GetProfilePicReq) (*pb.GetProfilePicRes, error) {
// 	fs.log.Infof("File server got request to retrieve profile pic")
// 	return &pb.GetProfilePicRes{}, nil
// }

// func (fs *FileServer) DeleteProfilePic(ctx context.Context, req *pb.DeleteProfilePicReq) (*pb.DeleteProfilePicRes, error) {
// 	fs.log.Infof("File server got request to delete profile pic")
// 	return &pb.DeleteProfilePicRes{}, nil
// }
