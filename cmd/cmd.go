package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/itsHabib/sim/internal/images"
	"github.com/itsHabib/sim/internal/images/service"
)

type Runner struct {
	logger           *zap.Logger
	root             *cobra.Command
	svc              *service.Service
	filePath         string
	fileName         string
	imageID          string
	downloadFilePath string
}

func NewRunner(logger *zap.Logger, svc *service.Service) *Runner {
	r := Runner{
		logger: logger,
		svc:    svc,
	}
	r.registerCommands()

	return &r
}

func (r *Runner) Run() error {
	return r.root.Execute()
}

func (r *Runner) registerCommands() {
	root := rootCmd()

	root.AddCommand(
		r.deleteCommand(),
		r.downloadCommand(),
		r.listCommand(),
		r.uploadCommand(),
	)

	r.root = root
}

func (r *Runner) deleteCommand() *cobra.Command {
	c := cobra.Command{
		Use:   "delete",
		Short: "Delete the image.",
		Args:  cobra.NoArgs,
		RunE:  r.runDeleteCommand,
	}

	c.Flags().StringVarP(&r.imageID, "imageId", "", "", "Id of the image to download (required)")
	c.MarkFlagRequired("imageId")

	return &c
}

func (r *Runner) downloadCommand() *cobra.Command {
	c := cobra.Command{
		Use:   "download",
		Short: "Download the image to the specified file path.",
		Args:  cobra.NoArgs,
		RunE:  r.runDownloadCommand,
	}

	c.Flags().StringVarP(&r.downloadFilePath, "file", "f", "", "Path to download the file into (required)")
	c.Flags().StringVarP(&r.imageID, "imageId", "", "", "Id of the image to download (required)")
	c.MarkFlagRequired("imageId")
	c.MarkFlagRequired("file")

	return &c
}

func (r *Runner) listCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all images",
		Args:  cobra.NoArgs,
		RunE:  r.runListCommand,
	}
}

func (r *Runner) uploadCommand() *cobra.Command {
	c := cobra.Command{
		Use:   "upload",
		Short: "Upload an image",
		Args:  cobra.NoArgs,
		RunE:  r.runUploadCommand,
	}
	c.Flags().StringVarP(&r.filePath, "file", "f", "", "Path to the image file (required)")
	c.Flags().StringVarP(&r.fileName, "name", "n", "", "Name for the image (required)")
	c.MarkFlagRequired("file")
	c.MarkFlagRequired("name")

	return &c
}

func (r *Runner) runDeleteCommand(cmd *cobra.Command, args []string) error {
	if err := r.svc.Delete(r.imageID); err != nil {
		return err
	}

	fmt.Printf("Image (%s) successfully deleted\n", r.imageID)
	r.logger.Info("Image deleted", zap.String("imageId", r.imageID))
	return nil
}

func (r *Runner) runDownloadCommand(cmd *cobra.Command, args []string) error {
	logger := r.logger.With(zap.String("filePath", r.downloadFilePath), zap.String("imageId", r.imageID))

	f, err := os.Create(r.downloadFilePath)
	if err != nil {
		const msg = "unable to create file"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	req := images.DownloadRequest{
		ID:     r.imageID,
		Stream: f,
	}

	if err := r.svc.Download(req); err != nil {
		const msg = "unable to download image"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	fmt.Printf("successfully downloaded file to: (%s)\n", r.downloadFilePath)

	return nil
}

func (r *Runner) runListCommand(cmd *cobra.Command, args []string) error {
	list, err := r.svc.List()
	switch err {
	case nil:
	case images.ErrRecordNotFound:
		fmt.Println("[]")
		return nil
	default:
		const msg = "failed to list images"
		r.logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	b, err := json.MarshalIndent(list, "", " ")
	if err != nil {
		const msg = "failed to marshal image list"
		r.logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	fmt.Println(string(b))

	return nil
}

func (r *Runner) runUploadCommand(cmd *cobra.Command, args []string) error {
	logger := r.logger.With(zap.String("filePath", r.filePath), zap.String("fileName", r.fileName))

	f, err := os.Open(r.filePath)
	if err != nil {
		const msg = "failed to open file"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	request := images.UploadRequest{
		Name: r.fileName,
		Body: f,
	}
	imageID, err := r.svc.Upload(request)
	if err != nil {
		const msg = "failed to upload file"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	fmt.Printf("Image uploaded successfully with id(%s)\n", imageID)

	return nil
}

func rootCmd() *cobra.Command {
	return &cobra.Command{
		Short: "A simple image manager",
		Long:  "A CLI for managing image files in cloud storage.",
	}
}
