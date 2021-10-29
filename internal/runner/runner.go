package runner

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/itsHabib/sim/internal/images"
	"github.com/itsHabib/sim/internal/images/service"
)

// Runner is responsible for running the cobra commands that interact
// with the images service.
type Runner struct {
	logger  *zap.Logger
	command *command
	svc     *service.Service
}

func NewRunner(logger *zap.Logger, svc *service.Service) *Runner {
	r := Runner{
		logger:  logger,
		svc:     svc,
		command: new(command),
	}
	r.registerCommands()

	return &r
}

// Run executes the underlying root cobra command.
func (r *Runner) Run() error {
	return r.command.root.Execute()
}

func (r *Runner) registerCommands() {
	r.command.root = rootCmd()

	r.command.root.AddCommand(
		r.deleteCommand(),
		r.downloadCommand(),
		r.listCommand(),
		r.uploadCommand(),
	)
}

func (r *Runner) deleteCommand() *cobra.Command {
	c := cobra.Command{
		Use:   "delete",
		Short: "Delete the image.",
		Args:  cobra.NoArgs,
		RunE:  r.runDeleteCommand,
	}

	c.Flags().StringVarP(&r.command.imageID, "imageId", "", "", "Id of the image to download (required)")
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

	c.Flags().StringVarP(&r.command.filePath, "file", "f", "", "Path to download the file into (required)")
	c.Flags().StringVarP(&r.command.imageID, "imageId", "", "", "Id of the image to download (required)")
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
	c.Flags().StringVarP(&r.command.filePath, "file", "f", "", "Path to the image file (required)")
	c.Flags().StringVarP(&r.command.imageName, "name", "n", "", "Name for the image (required)")
	c.MarkFlagRequired("file")
	c.MarkFlagRequired("name")

	return &c
}

func (r *Runner) runDeleteCommand(cmd *cobra.Command, args []string) error {
	logger := r.logger.With(zap.String("imageId", r.command.imageID))

	if err := r.svc.Delete(r.command.imageID); err != nil {
		const msg = "unable to delete image"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	logger.Debug("image deleted", zap.String("imageId", r.command.imageID))
	fmt.Printf("Image (%s) successfully deleted\n", r.command.imageID)

	return nil
}

func (r *Runner) runDownloadCommand(cmd *cobra.Command, args []string) error {
	logger := r.logger.With(zap.String("filePath", r.command.filePath), zap.String("imageId", r.command.imageID))

	f, err := os.Create(r.command.filePath)
	if err != nil {
		const msg = "unable to create file"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	req := images.DownloadRequest{
		ID:     r.command.filePath,
		Stream: f,
	}

	if err := r.svc.Download(req); err != nil {
		const msg = "unable to download image"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	logger.Debug("successfully downloaded image")
	fmt.Printf("successfully downloaded file to: (%s)\n", r.command.filePath)

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
	logger := r.logger.With(zap.String("filePath", r.command.filePath), zap.String("imageName", r.command.imageName))

	f, err := os.Open(r.command.filePath)
	if err != nil {
		const msg = "failed to open file"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	request := images.UploadRequest{
		Name: r.command.imageName,
		Body: f,
	}

	imageID, err := r.svc.Upload(request)
	if err != nil {
		const msg = "failed to upload file"
		logger.Error(msg, zap.Error(err))
		return fmt.Errorf(msg+": %w", err)
	}

	logger.Debug("successfully uploaded image")
	fmt.Printf("Image uploaded successfully with id(%s)\n", imageID)

	return nil
}

type command struct {
	root      *cobra.Command
	filePath  string
	imageName string
	imageID   string
}

func rootCmd() *cobra.Command {
	return &cobra.Command{
		Short: "A simple image manager",
		Long:  "A CLI for managing image files in cloud storage.",
	}
}
