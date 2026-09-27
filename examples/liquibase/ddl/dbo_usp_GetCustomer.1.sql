CREATE OR ALTER PROCEDURE dbo.usp_GetCustomer
(
    @CustomerId INT
)
AS
BEGIN
    SET NOCOUNT ON;

    SELECT
        CustomerId,
        CustomerName,
        EmailAddress
    FROM dbo.Customer
    WHERE CustomerId = @CustomerId;
END ;