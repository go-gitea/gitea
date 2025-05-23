package org.example;

import org.openqa.selenium.By;
import org.openqa.selenium.WebDriver;
import org.openqa.selenium.WebElement;
import org.openqa.selenium.interactions.Actions;
import org.openqa.selenium.support.PageFactory;
import org.openqa.selenium.support.ui.ExpectedConditions;
import org.openqa.selenium.support.ui.LoadableComponent;
import org.openqa.selenium.support.ui.Wait;
import org.openqa.selenium.support.ui.WebDriverWait;

import java.time.Duration;

import static org.junit.jupiter.api.Assertions.assertTrue;


public class NewProjectPage  extends LoadableComponent<NewProjectPage> {
    private WebDriver driver;
    private final String baseURL="http://localhost:3000/user/login";
    //private static final String baseURL1="https://a7bd-2a06-c701-78fb-bc00-e162-b721-5502-6b4b.ngrok-free.app/user/login";

    // Locators for New Project Fields
    private By titleField = By.id("_aria_auto_id_0");
    private By descriptionField = By.id("_combo_markdown_editor_1");
    private By templateDropdown =  By.id("_aria_auto_id_13");
    private By cardPreviewsDropdown = By.id("_aria_auto_id_17");
    private By createProjectButton = By.cssSelector(".ui.primary.button");
    private By cancelButton = By.cssSelector(".ui.cancel.button");

    // Locators for Description Buttons
     private By boldButton = By.cssSelector("md-bold.markdown-toolbar-button[aria-label='Add bold text']");
     private By numberProjects=By.cssSelector("div.ui.small.label");


    // Constructor
    public NewProjectPage(WebDriver driver) {
        this.driver = driver;

        // This call sets the WebElement fields.
        PageFactory.initElements(driver, this);
    }

    // Actions for New Project Fields
    public void enterTitle(String title) {
        driver.findElement(titleField).sendKeys(title);
    }

    public void enterDescription(String description) {
        Wait<WebDriver> wait = new WebDriverWait(driver, Duration.ofSeconds(5), Duration.ofMillis(500));
        wait.until(driver -> driver.findElement(descriptionField).isDisplayed());
        driver.findElement(descriptionField).sendKeys(description);
    }

    public void selectTemplate(String templateName) {
        Actions actions = new Actions(driver);

        // Click the dropdown
        WebElement dropdown = driver.findElement(By.xpath("//div[@class='ui selection dropdown']"));
        actions.moveToElement(dropdown).click().perform();

        // Click the option
        WebElement option = driver.findElement(By.xpath("//div[@role='option' and text()='" + templateName + "']"));
        actions.moveToElement(option).click().perform();
    }


    public void selectCardPreview(String cardPreviewOption) {

        Actions actions=new Actions(driver);

        WebElement dropdown= driver.findElement(By.xpath("//div[@class='ui selection dropdown']"));
        actions.moveToElement(dropdown).click().perform();

        WebElement option=driver.findElement(By.xpath("//div[@role='option' and text()='" + cardPreviewOption + "']"));
        actions.moveToElement(option).click().perform();

    }

    public void clickCreateProject() {
        driver.findElement(createProjectButton).click();
    }

    public void clickCancel() {
        driver.findElement(cancelButton).click();
    }

    // Actions for Description Buttons
    public void clickBoldButton() {
        driver.findElement(boldButton).click();
    }



    // Getter for Description Content
    public String getDescriptionContent() {
        WebElement previewElement = driver.findElement(descriptionField);
        String content = previewElement.getAttribute("value");
        return content;
    }
    public int getNumberOfProjects(){
        WebElement numberElement = driver.findElement(numberProjects);



            String numberText = numberElement.getText();
            int number = Integer.parseInt(numberText);
            return number;


    }

    public boolean isSuccessfulProjectPage(){


        return driver.getTitle().equals("Projects - Gitea: Git with a cup of tea");

    }

    @Override
    protected void load() {
        this.driver.manage().timeouts().implicitlyWait(Duration.ofSeconds(2));
        driver.get(baseURL+"/maias/-/projects/new");
    }

    @Override
    protected void isLoaded() throws Error {

        assertTrue(driver.getTitle().contains("New Project"));

    }
}
