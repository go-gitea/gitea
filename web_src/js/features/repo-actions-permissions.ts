// Repository Actions Permissions Settings
// Handles the UI interactions for the repository actions permissions page

export function initRepoActionsPermissions() {
    // Show/hide custom permissions based on mode selection
    $('.ui.dropdown').dropdown({
        onChange(value) {
            // Show custom options only when Custom mode is selected
            if (value === '2') {
                $('#custom-permissions').removeClass('hide');
            } else {
                $('#custom-permissions').addClass('hide');
            }
        },
    });

    // Warning when enabling write permissions
    // Helps prevent accidental security issues
    $('#contents_write, #packages_write').on('change', function () {
        if ($(this).is(':checked')) {
            // Log warning for write permissions being enabled
            console.log('Write permission enabled - ensure this is intentional');
        }
    });
}
